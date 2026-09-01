//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateKnowledgeGroupAction 修改知识库分组。
type UpdateKnowledgeGroupAction struct{ db *bun.DB }

// NewUpdateKnowledgeGroupAction 创建知识库分组修改操作。
func NewUpdateKnowledgeGroupAction(db *bun.DB) *UpdateKnowledgeGroupAction {
	return &UpdateKnowledgeGroupAction{db: db}
}

// Execute 修改分组名称或上级分组。
func (a *UpdateKnowledgeGroupAction) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID, groupID string, input GroupInput) (*Record, error) {
	input, fields := normalizeGroupInput(input)
	if len(fields) > 0 {
		return nil, &common.FieldError{Fields: fields}
	}
	var output *Record
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		group, err := loadKnowledgeGroup(ctx, tx, identity.Organization.ID, knowledgeBaseID, groupID)
		if err != nil {
			return err
		}
		if group.IsDefault {
			return ErrGroupInvalid
		}
		parentID, err := validGroupParent(ctx, tx, identity.Organization.ID, knowledgeBaseID, input.ParentID, group.ID)
		if err != nil {
			return err
		}
		if parentID != nil && group.ParentID == nil {
			exists, err := tx.NewSelect().TableExpr("knowledge_groups AS child").Where("child.parent_id = ?", group.ID).Exists(ctx)
			if err != nil {
				return err
			}
			if exists {
				return ErrGroupInvalid
			}
		}
		result, err := tx.NewUpdate().Model((*servermodels.KnowledgeGroup)(nil)).
			Set("name = ?", input.Name).
			Set("parent_id = ?", parentID).
			Set("updated_at = now()").
			Where("id = ?", group.ID).
			Where("knowledge_base_id = ?", knowledgeBaseID).
			Exec(ctx)
		if isConstraintConflict(err, "knowledge_groups_sibling_name_unique") {
			return &common.FieldError{Fields: map[string]common.FieldCode{"name": ValidationGroupNameDuplicate}}
		}
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
		return nil, fmt.Errorf("update knowledge group: %w", err)
	}
	return output, nil
}
