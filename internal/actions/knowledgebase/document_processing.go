//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"uuid"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/webfetch"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

const ProcessDocumentActionName = "knowledge.document.process"

// DocumentProcessing 安排文档处理和重新处理。
type DocumentProcessing struct {
	db    *bun.DB
	tasks servertask.TxEnqueuer
}

// NewDocumentProcessing 创建文档处理调度器。
func NewDocumentProcessing(db *bun.DB, tasks servertask.TxEnqueuer) *DocumentProcessing {
	return &DocumentProcessing{db: db, tasks: tasks}
}

// enqueue 固定当前分段和向量参数，并在业务事务中投递处理任务；fetchPage 表示网页来源本次是否重新抓取。
func (p *DocumentProcessing) enqueue(ctx context.Context, tx bun.IDB, organizationID string, base *servermodels.KnowledgeBase, document *servermodels.KnowledgeDocument, fetchPage bool) error {
	document.ProcessingID = uuid.NewV7().String()
	document.ChunkLength, document.ChunkOverlap = *base.ChunkLength, *base.ChunkOverlap
	document.EmbeddingProviderID, document.EmbeddingModelIdentifier, document.EmbeddingDimension = base.EmbeddingProviderID, base.EmbeddingModelIdentifier, base.EmbeddingDimension
	document.Status, document.FailureCode = domain.KnowledgeIndexQueued, ""
	if _, err := tx.NewUpdate().Model(document).Column("processing_id", "chunk_length", "chunk_overlap", "embedding_provider_id", "embedding_model_identifier", "embedding_dimension", "status", "failure_code").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
		return err
	}
	_, err := p.tasks.EnqueueIn(ctx, tx, ProcessDocumentActionName, ProcessInput{
		OrganizationID: organizationID, KnowledgeBaseID: base.ID, DocumentID: document.ID, SourceKind: document.SourceKind, FetchPage: fetchPage, ProcessingID: document.ProcessingID,
		ChunkLength: document.ChunkLength, ChunkOverlap: document.ChunkOverlap,
		EmbeddingProviderID: document.EmbeddingProviderID, EmbeddingModelIdentifier: document.EmbeddingModelIdentifier, EmbeddingDimension: document.EmbeddingDimension,
	}, servertask.EnqueueOptions{Queue: servertask.QueueKnowledge, MaxAttempts: 1, IdempotencyKey: document.ProcessingID, TriggerType: servertask.TriggerBusiness})
	return err
}

// Retry 按当前配置重新索引文档，网页来源读取已保存的快照。
func (p *DocumentProcessing) Retry(ctx context.Context, identity *servermodels.Identity, baseID, documentID string) error {
	return p.schedule(ctx, identity, baseID, documentID, "", false)
}

// Refetch 重新抓取网页文档，传入的地址非空时同时更新页面地址。
func (p *DocumentProcessing) Refetch(ctx context.Context, identity *servermodels.Identity, baseID, documentID, sourceURL string) error {
	return p.schedule(ctx, identity, baseID, documentID, sourceURL, true)
}

// schedule 在业务事务中锁定文档并投递一次处理。
func (p *DocumentProcessing) schedule(ctx context.Context, identity *servermodels.Identity, baseID, documentID, sourceURL string, fetchPage bool) error {
	return p.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
		if fetchPage {
			if document.SourceKind != domain.KnowledgeDocumentSourceWeb {
				return ErrDocumentSourceUnsupported
			}
			if address := strings.TrimSpace(sourceURL); address != "" {
				normalized, err := webfetch.Normalize(address)
				if err != nil {
					return &common.FieldError{Fields: map[string]common.FieldCode{"sourceUrl": ValidationDocumentURLInvalid}}
				}
				if _, err := tx.NewUpdate().Model(document).Set("source_url = ?", normalized).Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
					if isConstraintConflict(err, "knowledge_documents_source_url_unique") {
						return ErrDocumentURLDuplicate
					}
					return err
				}
			}
		}
		return p.enqueue(ctx, tx, identity.Organization.ID, base, document, fetchPage)
	})
}
