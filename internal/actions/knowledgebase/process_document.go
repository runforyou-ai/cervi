//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/textsplit"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/documentconvert"
	"github.com/runforyou-ai/cervi/internal/integration/embedding"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
	"uuid"
)

// segmentInsertSize 是单条插入语句写入的分段数量，受 PostgreSQL 绑定参数上限约束。
const segmentInsertSize = 500

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
	current, err := a.setStage(ctx, input, domain.KnowledgeDocumentFetching)
	if err != nil || !current {
		return err
	}
	slog.Info("知识文档处理开始", "document_id", input.DocumentID, "processing_id", input.ProcessingID)
	file := &servermodels.File{}
	err = a.db.NewSelect().Model(file).Join("JOIN knowledge_documents kd ON kd.file_id = f.id").Where("kd.id = ? AND f.organization_id = ?", input.DocumentID, input.OrganizationID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return &ProcessError{Code: "file_read_failed", Stage: domain.KnowledgeDocumentFetching}
	}
	if err != nil {
		return err
	}
	// 按任务快照读取供应商并解析 OpenAI 兼容入口。
	provider := &servermodels.AIProvider{}
	err = a.db.NewSelect().Model(provider).Where("id = ? AND organization_id = ?", input.EmbeddingProviderID, input.OrganizationID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return &ProcessError{Code: "embedding_model_unavailable", Stage: domain.KnowledgeDocumentEmbedding}
	}
	if err != nil {
		return err
	}
	baseURL, err := common.CompatibleModelBaseURL(provider.Brand, provider.APIURL)
	if err != nil {
		return &ProcessError{Code: "embedding_model_unavailable", Stage: domain.KnowledgeDocumentEmbedding}
	}
	source, err := a.files.Open(ctx, file)
	if err != nil {
		return &ProcessError{Code: "file_read_failed", Stage: domain.KnowledgeDocumentFetching}
	}
	defer source.Close()

	if current, err := a.setStage(ctx, input, domain.KnowledgeDocumentConverting); err != nil || !current {
		return err
	}
	markdown, err := a.converter.Convert(ctx, file.OriginalName, source)
	if err != nil {
		var failure *documentconvert.Error
		if errors.As(err, &failure) {
			return &ProcessError{Code: failure.Code, Stage: domain.KnowledgeDocumentConverting}
		}
		return err
	}

	if current, err := a.setStage(ctx, input, domain.KnowledgeDocumentSplitting); err != nil || !current {
		return err
	}
	segments := textsplit.Split(markdown, input.ChunkLength, input.ChunkOverlap)
	if len(segments) == 0 {
		return &ProcessError{Code: "empty_content", Stage: domain.KnowledgeDocumentSplitting}
	}

	if current, err := a.setStage(ctx, input, domain.KnowledgeDocumentEmbedding); err != nil || !current {
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
			return &ProcessError{Code: failure.Code, Stage: domain.KnowledgeDocumentEmbedding}
		}
		return err
	}

	if len(vectors) != len(segments) {
		return &ProcessError{Code: "embedding_failed", Stage: domain.KnowledgeDocumentEmbedding}
	}

	if current, err := a.setStage(ctx, input, domain.KnowledgeDocumentPublishing); err != nil || !current {
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
		if _, err := tx.NewDelete().TableExpr("public.knowledge_segments").Where("meta->>'document_id' = ?", input.DocumentID).Exec(ctx); err != nil {
			return err
		}
		if err := insertSegments(ctx, tx, input, segments, vectors); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Model(document).Set("status = ?", domain.KnowledgeDocumentSucceeded).Set("segment_batch_id = ?", input.ProcessingID).Set("segment_count = ?", len(segments)).Set("failure_code = ''").Set("updated_at = now()").WherePK().Exec(ctx)
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
func (a *ProcessDocumentAction) setStage(ctx context.Context, input ProcessInput, stage domain.KnowledgeDocumentStatus) (bool, error) {
	result, err := a.db.NewUpdate().Model((*servermodels.KnowledgeDocument)(nil)).
		Set("status = ?", stage).Set("failure_code = ''").Set("updated_at = now()").
		Where("id = ? AND processing_id = ? AND status NOT IN (?, ?, ?, ?)", input.DocumentID, input.ProcessingID,
			domain.KnowledgeDocumentInitial, domain.KnowledgeDocumentSucceeded, domain.KnowledgeDocumentFailed, domain.KnowledgeDocumentCancelled).
		Exec(ctx)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

// insertSegments 按任务标识和文档内序号写入本批次分段及其向量。
func insertSegments(ctx context.Context, tx bun.Tx, input ProcessInput, segments []textsplit.Segment, vectors [][]float32) error {
	namespace, err := uuid.Parse(input.ProcessingID)
	if err != nil {
		return err
	}
	for start := 0; start < len(segments); start += segmentInsertSize {
		batch := segments[start:min(start+segmentInsertSize, len(segments))]
		placeholders := make([]string, 0, len(batch))
		arguments := make([]any, 0, len(batch)*5)
		for offset, segment := range batch {
			vector := make([]string, 0, input.EmbeddingDimension)
			for _, value := range vectors[start+offset] {
				vector = append(vector, strconv.FormatFloat(float64(value), 'f', -1, 32))
			}
			meta := map[string]any{
				"organization_id": input.OrganizationID, "knowledge_base_id": input.KnowledgeBaseID,
				"document_id": input.DocumentID, "batch_id": input.ProcessingID,
				"position": segment.Position, "character_count": segment.CharacterCount,
			}
			encoded, err := json.Marshal(meta)
			if err != nil {
				return err
			}
			placeholders = append(placeholders, "(?, ?, ?::jsonb, ?::vector, ?)")
			arguments = append(arguments, common.NewUUIDv5(namespace, strconv.Itoa(segment.Position)).String(),
				segment.Content, string(encoded), "["+strings.Join(vector, ",")+"]", input.EmbeddingDimension)
		}
		query := "INSERT INTO public.knowledge_segments (id, content, meta, embedding, embedding_dimension) VALUES " + strings.Join(placeholders, ", ")
		if _, err := tx.ExecContext(ctx, query, arguments...); err != nil {
			return err
		}
	}
	return nil
}

// FinalizeFailure 保存当前文档任务的失败状态和原因码。
func (a *ProcessDocumentAction) FinalizeFailure(ctx context.Context, input ProcessInput, runErr error) error {
	code := "service_failed"
	var stage domain.KnowledgeDocumentStatus
	var failure *ProcessError
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
