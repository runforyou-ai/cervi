//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
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
		if _, err := loadKnowledgeBase(ctx, tx, identity.Organization.ID, knowledgeBaseID); err != nil {
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
