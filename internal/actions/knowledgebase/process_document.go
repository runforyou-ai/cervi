//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeprocessing"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type documentProcessor interface {
	Process(context.Context, knowledgeprocessing.ProcessInput, string, io.Reader) (knowledgeprocessing.ProcessResult, error)
}
type documentFileReader interface {
	Open(context.Context, *servermodels.File) (io.ReadCloser, error)
}

// ProcessDocumentAction 执行原件读取、远端分段与结果发布。
type ProcessDocumentAction struct {
	db        *bun.DB
	processor documentProcessor
	files     documentFileReader
}

// NewProcessDocumentAction 创建文档处理任务。
func NewProcessDocumentAction(db *bun.DB, processor documentProcessor, files documentFileReader) *ProcessDocumentAction {
	return &ProcessDocumentAction{db: db, processor: processor, files: files}
}

// Execute 执行当前文档任务，并在远端完整写入后发布分段批次。
func (a *ProcessDocumentAction) Execute(ctx context.Context, input knowledgeprocessing.ProcessInput) error {
	started := time.Now()
	result, err := a.db.NewUpdate().Model((*servermodels.KnowledgeDocument)(nil)).Set("status = ?", domain.KnowledgeDocumentFetching).Set("failure_code = ''").Set("updated_at = now()").Where("id = ? AND processing_id = ? AND status NOT IN (?, ?, ?, ?)", input.DocumentID, input.ProcessingID, domain.KnowledgeDocumentInitial, domain.KnowledgeDocumentSucceeded, domain.KnowledgeDocumentFailed, domain.KnowledgeDocumentCancelled).Exec(ctx)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count == 0 {
		return err
	}
	slog.Info("知识文档处理开始", "document_id", input.DocumentID, "processing_id", input.ProcessingID)
	file := &servermodels.File{}
	err = a.db.NewSelect().Model(file).Join("JOIN knowledge_documents kd ON kd.file_id = f.id").Where("kd.id = ? AND f.organization_id = ?", input.DocumentID, input.OrganizationID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return &knowledgeprocessing.Error{Code: "file_read_failed", Stage: domain.KnowledgeDocumentFetching}
	}
	if err != nil {
		return err
	}
	source, err := a.files.Open(ctx, file)
	if err != nil {
		return &knowledgeprocessing.Error{Code: "file_read_failed", Stage: domain.KnowledgeDocumentFetching}
	}
	defer source.Close()
	output, err := a.processor.Process(ctx, input, file.OriginalName, source)
	if err != nil {
		return err
	}
	if output.Stale {
		return nil
	}
	published := false
	// 持有文档行锁校验当前任务并发布分段批次。
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		document := &servermodels.KnowledgeDocument{}
		if err := tx.NewSelect().Model(document).Where("kd.id = ?", input.DocumentID).For("UPDATE").Scan(ctx); errors.Is(err, sql.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		if document.ProcessingID != input.ProcessingID || !document.Status.IsProcessing() {
			return nil
		}
		count, err := tx.NewSelect().TableExpr("public.knowledge_segments").Where("meta->>'document_id' = ? AND meta->>'batch_id' = ?", input.DocumentID, input.ProcessingID).Count(ctx)
		if err != nil {
			return err
		}
		if count != output.SegmentCount {
			return &knowledgeprocessing.Error{Code: "service_failed"}
		}
		_, err = tx.NewUpdate().Model(document).Set("status = ?", domain.KnowledgeDocumentSucceeded).Set("segment_batch_id = ?", input.ProcessingID).Set("segment_count = ?", count).Set("failure_code = ''").Set("updated_at = now()").WherePK().Exec(ctx)
		if err != nil {
			return err
		}
		// 在发布事务中清理旧批次的分段。
		if _, err := tx.NewDelete().TableExpr("public.knowledge_segments").Where("meta->>'document_id' = ? AND meta->>'batch_id' <> ?", input.DocumentID, input.ProcessingID).Exec(ctx); err != nil {
			return err
		}
		if err := servertask.LockExecution(ctx, tx); err != nil {
			return err
		}
		published = true
		return nil
	})
	if err == nil && published {
		slog.Info("知识文档分段完成", "document_id", input.DocumentID, "processing_id", input.ProcessingID, "segment_count", output.SegmentCount, "duration_ms", time.Since(started).Milliseconds())
	}
	return err
}

// FinalizeFailure 保存当前文档任务的失败状态和原因码。
func (a *ProcessDocumentAction) FinalizeFailure(ctx context.Context, input knowledgeprocessing.ProcessInput, runErr error) error {
	code := "service_failed"
	var stage domain.KnowledgeDocumentStatus
	var failure *knowledgeprocessing.Error
	if errors.As(runErr, &failure) {
		code, stage = failure.Code, failure.Stage
	}
	changed := false
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		query := tx.NewUpdate().Model((*servermodels.KnowledgeDocument)(nil)).Set("status = ?", domain.KnowledgeDocumentFailed).Set("failure_code = ?", code).Set("updated_at = now()").Where("id = ? AND processing_id = ? AND status NOT IN (?, ?, ?, ?)", input.DocumentID, input.ProcessingID, domain.KnowledgeDocumentInitial, domain.KnowledgeDocumentSucceeded, domain.KnowledgeDocumentFailed, domain.KnowledgeDocumentCancelled)
		result, err := query.Exec(ctx)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		changed = count > 0
		return servertask.LockExecution(ctx, tx)
	})
	if errors.Is(err, servertask.ErrExecutionLost) {
		return nil
	}

	if err == nil && changed {
		slog.Warn("知识文档处理失败", "document_id", input.DocumentID, "processing_id", input.ProcessingID, "failure_code", code, "stage", stage)
	}
	return err
}
