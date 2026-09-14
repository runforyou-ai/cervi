//go:build server

package knowledgebase

import (
	"context"
	"uuid"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// QAProcessing 安排问答条目的索引和重新索引。
type QAProcessing struct {
	db    *bun.DB
	tasks servertask.TxEnqueuer
}

// NewQAProcessing 创建问答索引调度器。
func NewQAProcessing(db *bun.DB, tasks servertask.TxEnqueuer) *QAProcessing {
	return &QAProcessing{db: db, tasks: tasks}
}

// enqueue 固定当前向量参数，并在业务事务中投递索引任务。
func (p *QAProcessing) enqueue(ctx context.Context, tx bun.IDB, organizationID string, base *servermodels.KnowledgeBase, entry *servermodels.KnowledgeQAEntry) error {
	entry.ProcessingID = uuid.NewV7().String()
	entry.EmbeddingProviderID, entry.EmbeddingModelIdentifier, entry.EmbeddingDimension = base.EmbeddingProviderID, base.EmbeddingModelIdentifier, base.EmbeddingDimension
	entry.Status, entry.FailureCode = domain.KnowledgeIndexQueued, ""
	if _, err := tx.NewUpdate().Model(entry).Column("processing_id", "embedding_provider_id", "embedding_model_identifier", "embedding_dimension", "status", "failure_code").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
		return err
	}
	_, err := p.tasks.EnqueueIn(ctx, tx, ProcessQAEntryActionName, ProcessQAInput{
		OrganizationID: organizationID, KnowledgeBaseID: base.ID, EntryID: entry.ID, ProcessingID: entry.ProcessingID,
		EmbeddingProviderID: entry.EmbeddingProviderID, EmbeddingModelIdentifier: entry.EmbeddingModelIdentifier, EmbeddingDimension: entry.EmbeddingDimension,
	}, servertask.EnqueueOptions{Queue: servertask.QueueKnowledge, MaxAttempts: 1, IdempotencyKey: entry.ProcessingID, TriggerType: servertask.TriggerBusiness})
	return err
}

// Retry 按当前知识库配置为问答条目投递一次索引任务。
func (p *QAProcessing) Retry(ctx context.Context, identity *servermodels.Identity, baseID, entryID string) error {
	return p.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		base, err := lockKnowledgeBase(ctx, tx, identity.Organization.ID, baseID)
		if err != nil {
			return err
		}
		if err := validateQAKnowledgeBase(base); err != nil {
			return err
		}
		entry, err := lockQAEntry(ctx, tx, baseID, entryID)
		if err != nil {
			return err
		}
		return p.enqueue(ctx, tx, identity.Organization.ID, base, entry)
	})
}
