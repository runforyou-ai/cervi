//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"uuid"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeprocessing"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

const ProcessDocumentActionName = "knowledge.document.process"

var (
	ErrSegmentsNotReady    = errors.New("knowledge document segments are not ready")
	ErrSegmentStale        = errors.New("knowledge document segment is stale")
	ErrSegmentQueryInvalid = errors.New("knowledge segment query is invalid")
)

// DocumentProcessing 安排文档处理和重新处理。
type DocumentProcessing struct {
	db    *bun.DB
	tasks servertask.TxEnqueuer
}

// NewDocumentProcessing 创建文档处理调度器。
func NewDocumentProcessing(db *bun.DB, tasks servertask.TxEnqueuer) *DocumentProcessing {
	return &DocumentProcessing{db: db, tasks: tasks}
}

// enqueue 固定当前分段和向量参数，并在业务事务中投递处理任务。
func (p *DocumentProcessing) enqueue(ctx context.Context, tx bun.IDB, organizationID string, base *servermodels.KnowledgeBase, document *servermodels.KnowledgeDocument) error {
	document.ProcessingID = uuid.NewV7().String()
	document.ChunkLength, document.ChunkOverlap = *base.ChunkLength, *base.ChunkOverlap
	document.EmbeddingProviderID, document.EmbeddingModelIdentifier, document.EmbeddingDimension = base.EmbeddingProviderID, base.EmbeddingModelIdentifier, base.EmbeddingDimension
	document.Status, document.FailureCode = domain.KnowledgeDocumentQueued, ""
	if _, err := tx.NewUpdate().Model(document).Column("processing_id", "chunk_length", "chunk_overlap", "embedding_provider_id", "embedding_model_identifier", "embedding_dimension", "status", "failure_code").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
		return err
	}
	// 保留已发布分段，清理被新任务替代但尚未发布的批次。
	if _, err := tx.NewDelete().TableExpr("public.knowledge_segments").Where("meta->>'document_id' = ? AND meta->>'batch_id' <> ?", document.ID, document.SegmentBatchID).Exec(ctx); err != nil {
		return err
	}
	_, err := p.tasks.EnqueueIn(ctx, tx, ProcessDocumentActionName, knowledgeprocessing.ProcessInput{
		OrganizationID: organizationID, KnowledgeBaseID: base.ID, DocumentID: document.ID, ProcessingID: document.ProcessingID,
		ChunkLength: document.ChunkLength, ChunkOverlap: document.ChunkOverlap,
		EmbeddingProviderID: document.EmbeddingProviderID, EmbeddingModelIdentifier: document.EmbeddingModelIdentifier, EmbeddingDimension: document.EmbeddingDimension,
	}, servertask.EnqueueOptions{Queue: servertask.QueueKnowledge, MaxAttempts: 1, IdempotencyKey: document.ProcessingID, TriggerType: servertask.TriggerBusiness})
	return err
}

// Retry 检查服务连接后按当前配置投递一次处理，连接失败直接保存失败状态。
func (p *DocumentProcessing) Retry(ctx context.Context, identity *servermodels.Identity, baseID, documentID string, checkConnection func(context.Context) error) error {
	var connectionErr error
	err := p.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		base, err := lockKnowledgeBase(ctx, tx, identity.Organization.ID, baseID)
		if err != nil {
			return err
		}
		document := &servermodels.KnowledgeDocument{}
		err = tx.NewSelect().Model(document).Where("kd.id = ? AND kd.knowledge_base_id = ?", documentID, baseID).For("UPDATE").Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDocumentNotFound
		}
		if err != nil {
			return err
		}
		connectionErr = checkConnection(ctx)
		if connectionErr != nil {
			code := "unavailable"
			var failure *knowledgeprocessing.Error
			if errors.As(connectionErr, &failure) {
				code = failure.Code
			}
			// 生成新的处理标识并保存连接失败状态。
			_, err := tx.NewUpdate().Model(document).Set("processing_id = ?", uuid.NewV7().String()).Set("status = ?", domain.KnowledgeDocumentFailed).Set("failure_code = ?", code).Set("updated_at = now()").WherePK().Exec(ctx)
			return err
		}
		return p.enqueue(ctx, tx, identity.Organization.ID, base, document)
	})
	if err != nil {
		return err
	}
	if connectionErr != nil {
		slog.Warn("知识文档处理服务连接失败", "document_id", documentID, "error", connectionErr)
	}
	return connectionErr
}
