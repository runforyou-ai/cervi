//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteKnowledgeGroupAction 删除知识库空分组。
type DeleteKnowledgeGroupAction struct{ db *bun.DB }

// NewDeleteKnowledgeGroupAction 创建知识库分组删除操作。
func NewDeleteKnowledgeGroupAction(db *bun.DB) *DeleteKnowledgeGroupAction {
	return &DeleteKnowledgeGroupAction{db: db}
}

// Execute 删除不含子分组的非默认分组。
func (a *DeleteKnowledgeGroupAction) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID, groupID string) (*Record, error) {
	var output *Record
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		group, err := loadKnowledgeGroup(ctx, tx, identity.Organization.ID, knowledgeBaseID, groupID)
		if err != nil {
			return err
		}
		if group.IsDefault {
			return ErrGroupInvalid
		}
		occupied, err := tx.NewSelect().TableExpr("knowledge_groups AS child").Where("child.parent_id = ?", group.ID).Exists(ctx)
		if err != nil {
			return err
		}
		if occupied {
			return ErrGroupNotEmpty
		}
		result, err := tx.NewDelete().Model((*servermodels.KnowledgeGroup)(nil)).
			Where("id = ?", group.ID).
			Where("knowledge_base_id = ?", knowledgeBaseID).
			Exec(ctx)
		if err != nil {
			return err
		}
		if err := rowsAffectedOne(result, ErrGroupNotFound); err != nil {
			return err
		}
		output, err = loadKnowledgeBaseRecord(ctx, tx, identity.Organization.ID, knowledgeBaseID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("delete knowledge group: %w", err)
	}
	return output, nil
}
