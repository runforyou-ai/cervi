//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteKnowledgeBaseAction 删除企业知识库。
type DeleteKnowledgeBaseAction struct {
	db *bun.DB
}

// NewDeleteKnowledgeBaseAction 创建知识库删除操作。
func NewDeleteKnowledgeBaseAction(db *bun.DB) *DeleteKnowledgeBaseAction {
	return &DeleteKnowledgeBaseAction{db: db}
}

// Execute 删除当前企业中的指定知识库。
func (a *DeleteKnowledgeBaseAction) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID string) error {
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := lockKnowledgeBase(ctx, tx, identity.Organization.ID, knowledgeBaseID); err != nil {
			return err
		}
		// 文档原件在事务中释放，后台文件任务负责实际清理。
		if _, err := tx.NewUpdate().Model((*servermodels.File)(nil)).Set("status = ?", domain.FileStatusDeleting).Set("expires_at = now()").Set("updated_at = now()").Where("id IN (SELECT file_id FROM knowledge_documents WHERE knowledge_base_id = ?)", knowledgeBaseID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*servermodels.KnowledgeDocument)(nil)).Where("knowledge_base_id = ?", knowledgeBaseID).Exec(ctx); err != nil {
			return err
		}
		// 先删除内容，再删除条目和分组，全部操作持有知识库锁。
		entries := tx.NewSelect().Model((*servermodels.KnowledgeQAEntry)(nil)).Column("id").Where("knowledge_base_id = ?", knowledgeBaseID)
		if _, err := tx.NewDelete().Model((*servermodels.KnowledgeQAContent)(nil)).Where("entry_id IN (?)", entries).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*servermodels.KnowledgeQAEntry)(nil)).Where("knowledge_base_id = ?", knowledgeBaseID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*servermodels.KnowledgeGroup)(nil)).Where("knowledge_base_id = ?", knowledgeBaseID).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewDelete().Model((*servermodels.KnowledgeBase)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", knowledgeBaseID).
			Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("delete knowledge base: %w", err)
	}
	return nil
}
