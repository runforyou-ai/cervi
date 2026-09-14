//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/textsplit"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/documentconvert"
	"github.com/runforyou-ai/cervi/internal/integration/embedding"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type documentConverter interface {
	Convert(context.Context, string, io.Reader) (string, error)
}
type segmentEmbedder interface {
	Embed(context.Context, embedding.Credential, string, int, []string) ([][]float32, error)
}
type documentFileReader interface {
	Open(context.Context, *servermodels.File) (io.ReadCloser, error)
}

// ProcessDocumentAction 执行原件读取、转换、分段、向量化与批次发布。
type ProcessDocumentAction struct {
	db        *bun.DB
	converter documentConverter
	embedder  segmentEmbedder
	files     documentFileReader
}

// NewProcessDocumentAction 创建文档处理任务。
func NewProcessDocumentAction(db *bun.DB, converter documentConverter, embedder segmentEmbedder, files documentFileReader) *ProcessDocumentAction {
	return &ProcessDocumentAction{db: db, converter: converter, embedder: embedder, files: files}
}

// Execute 执行当前文档任务，并在同一事务中写入分段与发布批次。
func (a *ProcessDocumentAction) Execute(ctx context.Context, input ProcessInput) error {
	started := time.Now()
	current, err := a.setStage(ctx, input, domain.KnowledgeIndexFetching)
	if err != nil || !current {
		return err
	}
	slog.Info("知识文档处理开始", "document_id", input.DocumentID, "processing_id", input.ProcessingID)
	file := &servermodels.File{}
	err = a.db.NewSelect().Model(file).Join("JOIN knowledge_documents kd ON kd.file_id = f.id").Where("kd.id = ? AND f.organization_id = ?", input.DocumentID, input.OrganizationID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return &ProcessError{Code: "file_read_failed", Stage: domain.KnowledgeIndexFetching}
	}
	if err != nil {
		return err
	}
	// 按任务快照读取供应商并解析 OpenAI 兼容入口。
	provider := &servermodels.AIProvider{}
	err = a.db.NewSelect().Model(provider).Where("id = ? AND organization_id = ?", input.EmbeddingProviderID, input.OrganizationID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return &ProcessError{Code: "embedding_model_unavailable", Stage: domain.KnowledgeIndexEmbedding}
	}
	if err != nil {
		return err
	}
	baseURL, err := common.CompatibleModelBaseURL(provider.Brand, provider.APIURL)
	if err != nil {
		return &ProcessError{Code: "embedding_model_unavailable", Stage: domain.KnowledgeIndexEmbedding}
	}
	source, err := a.files.Open(ctx, file)
	if err != nil {
		return &ProcessError{Code: "file_read_failed", Stage: domain.KnowledgeIndexFetching}
	}
	defer source.Close()

	if current, err := a.setStage(ctx, input, domain.KnowledgeIndexConverting); err != nil || !current {
		return err
	}
	markdown, err := a.converter.Convert(ctx, file.OriginalName, source)
	if err != nil {
		var failure *documentconvert.Error
		if errors.As(err, &failure) {
			return &ProcessError{Code: failure.Code, Stage: domain.KnowledgeIndexConverting}
		}
		return err
	}

	if current, err := a.setStage(ctx, input, domain.KnowledgeIndexSplitting); err != nil || !current {
		return err
	}
	segments := textsplit.Split(markdown, input.ChunkLength, input.ChunkOverlap)
	if len(segments) == 0 {
		return &ProcessError{Code: "empty_content", Stage: domain.KnowledgeIndexSplitting}
	}

	if current, err := a.setStage(ctx, input, domain.KnowledgeIndexEmbedding); err != nil || !current {
		return err
	}
	contents := make([]string, 0, len(segments))
	for _, segment := range segments {
		contents = append(contents, segment.Content)
	}
	vectors, err := a.embedder.Embed(ctx, embedding.Credential{BaseURL: baseURL, APIKey: provider.APIKey}, input.EmbeddingModelIdentifier, input.EmbeddingDimension, contents)
	if err != nil {
		var failure *embedding.Error
		if errors.As(err, &failure) {
			return &ProcessError{Code: failure.Code, Stage: domain.KnowledgeIndexEmbedding}
		}
		return err
	}

	if len(vectors) != len(segments) {
		return &ProcessError{Code: "embedding_failed", Stage: domain.KnowledgeIndexEmbedding}
	}

	if current, err := a.setStage(ctx, input, domain.KnowledgeIndexPublishing); err != nil || !current {
		return err
	}
	published := false
	// 持有文档行锁校验当前任务，并在同一事务中写入分段与发布批次。
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
		if err := deleteSourceSegments(ctx, tx, input.DocumentID); err != nil {
			return err
		}
		batch := segmentBatch{OrganizationID: input.OrganizationID, KnowledgeBaseID: input.KnowledgeBaseID, SourceType: domain.KnowledgeSourceDocument, SourceID: input.DocumentID, BatchID: input.ProcessingID, EmbeddingDimension: input.EmbeddingDimension}
		if err := insertSegments(ctx, tx, batch, segments, vectors); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Model(document).Set("status = ?", domain.KnowledgeIndexSucceeded).Set("segment_batch_id = ?", input.ProcessingID).Set("segment_count = ?", len(segments)).Set("failure_code = ''").Set("updated_at = now()").WherePK().Exec(ctx)
		if err != nil {
			return err
		}
		if err := servertask.LockExecution(ctx, tx); err != nil {
			return err
		}
		published = true
		return nil
	})
	if err == nil && published {
		slog.Info("知识文档分段与向量完成", "document_id", input.DocumentID, "processing_id", input.ProcessingID, "segment_count", len(segments), "embedding_dimension", input.EmbeddingDimension, "duration_ms", time.Since(started).Milliseconds())
	}
	return err
}

// setStage 更新当前任务的执行阶段，任务已被替代或已进入终态时返回 false。
func (a *ProcessDocumentAction) setStage(ctx context.Context, input ProcessInput, stage domain.KnowledgeIndexStatus) (bool, error) {
	result, err := a.db.NewUpdate().Model((*servermodels.KnowledgeDocument)(nil)).
		Set("status = ?", stage).Set("failure_code = ''").Set("updated_at = now()").
		Where("id = ? AND processing_id = ? AND status NOT IN (?, ?, ?, ?)", input.DocumentID, input.ProcessingID,
			domain.KnowledgeIndexInitial, domain.KnowledgeIndexSucceeded, domain.KnowledgeIndexFailed, domain.KnowledgeIndexCancelled).
		Exec(ctx)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

// FinalizeFailure 保存当前文档任务的失败状态和原因码。
func (a *ProcessDocumentAction) FinalizeFailure(ctx context.Context, input ProcessInput, runErr error) error {
	code := "service_failed"
	var stage domain.KnowledgeIndexStatus
	var failure *ProcessError
	if errors.As(runErr, &failure) {
		code, stage = failure.Code, failure.Stage
	}
	changed := false
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		query := tx.NewUpdate().Model((*servermodels.KnowledgeDocument)(nil)).Set("status = ?", domain.KnowledgeIndexFailed).Set("failure_code = ?", code).Set("updated_at = now()").Where("id = ? AND processing_id = ? AND status NOT IN (?, ?, ?, ?)", input.DocumentID, input.ProcessingID, domain.KnowledgeIndexInitial, domain.KnowledgeIndexSucceeded, domain.KnowledgeIndexFailed, domain.KnowledgeIndexCancelled)
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
