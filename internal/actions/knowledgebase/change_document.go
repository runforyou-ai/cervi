//go:build server

package knowledgebase

import (
	"context"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// MoveDocumentAction 将文档移动到同一知识库中的分组。
type MoveDocumentAction struct{ db *bun.DB }

// NewMoveDocumentAction 创建文档移动操作。
func NewMoveDocumentAction(db *bun.DB) *MoveDocumentAction { return &MoveDocumentAction{db: db} }

// Execute 在知识库锁内修改文档分组，保留创建时间。
func (a *MoveDocumentAction) Execute(ctx context.Context, identity *servermodels.Identity, baseID, documentID, groupID string) error {
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := lockKnowledgeBase(ctx, tx, identity.Organization.ID, baseID); err != nil {
			return err
		}
		if _, err := loadDocumentRecord(ctx, tx, baseID, documentID); err != nil {
			return err
		}
		if _, err := loadKnowledgeGroup(ctx, tx, identity.Organization.ID, baseID, groupID); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Model((*servermodels.KnowledgeDocument)(nil)).Set("group_id = ?", groupID).Set("updated_at = now()").Where("id = ?", documentID).Exec(ctx)
		return err
	})
}

// DeleteDocumentAction 删除文档并释放原件。
type DeleteDocumentAction struct{ db *bun.DB }

// NewDeleteDocumentAction 创建文档删除操作。
func NewDeleteDocumentAction(db *bun.DB) *DeleteDocumentAction { return &DeleteDocumentAction{db: db} }

// Execute 将原件标记为待清理并删除文档关联。
func (a *DeleteDocumentAction) Execute(ctx context.Context, identity *servermodels.Identity, baseID, documentID string) error {
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := lockKnowledgeBase(ctx, tx, identity.Organization.ID, baseID); err != nil {
			return err
		}
		if _, err := loadDocumentRecord(ctx, tx, baseID, documentID); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*servermodels.File)(nil)).Set("status = ?", domain.FileStatusDeleting).Set("expires_at = now()").Set("updated_at = now()").Where("id IN (SELECT file_id FROM knowledge_documents WHERE id = ?)", documentID).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewDelete().Model((*servermodels.KnowledgeDocument)(nil)).Where("id = ?", documentID).Exec(ctx)
		return err
	})
}
