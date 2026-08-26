//go:build server

package knowledgebase

import (
	"context"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateKnowledgeGroupAction 创建知识库分组。
type CreateKnowledgeGroupAction struct{ db *bun.DB }

// NewCreateKnowledgeGroupAction 创建知识库分组新增操作。
func NewCreateKnowledgeGroupAction(db *bun.DB) *CreateKnowledgeGroupAction {
	return &CreateKnowledgeGroupAction{db: db}
}

// Execute 在知识库中创建最多两级的分组。
func (a *CreateKnowledgeGroupAction) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID string, input GroupInput) (*Record, error) {
	input, fields := normalizeGroupInput(input)
	if len(fields) > 0 {
		return nil, &common.FieldError{Fields: fields}
	}
	var output *Record
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		knowledgeBase, err := loadKnowledgeBase(ctx, tx, identity.Organization.ID, knowledgeBaseID)
		if err != nil {
			return err
		}
		if knowledgeBase.IntegrationConnectionID != nil {
			return ErrExternalGroupUnsupported
		}
		parentID, err := validGroupParent(ctx, tx, identity.Organization.ID, knowledgeBaseID, input.ParentID, "")
		if err != nil {
			return err
		}
		sortOrder, err := nextGroupSortOrder(ctx, tx, knowledgeBaseID, parentID)
		if err != nil {
			return err
		}
		group := &servermodels.KnowledgeGroup{KnowledgeBaseID: knowledgeBaseID, ParentID: parentID, Name: input.Name, SortOrder: sortOrder}
		_, err = tx.NewInsert().Model(group).
			Column("knowledge_base_id", "parent_id", "name", "sort_order").
			Exec(ctx)
		if isConstraintConflict(err, "knowledge_groups_sibling_name_unique") {
			return &common.FieldError{Fields: map[string]common.FieldCode{"name": ValidationGroupNameDuplicate}}
		}
		if err != nil {
			return err
		}
		output, err = loadKnowledgeBaseRecord(ctx, tx, identity.Organization.ID, knowledgeBaseID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create knowledge group: %w", err)
	}
	return output, nil
}

// validGroupParent 校验上级分组并返回可写入的编号。
func validGroupParent(ctx context.Context, db bun.IDB, organizationID, knowledgeBaseID, parentID, groupID string) (*string, error) {
	if parentID == "" {
		return nil, nil
	}
	if parentID == groupID {
		return nil, ErrGroupInvalid
	}
	parent, err := loadKnowledgeGroup(ctx, db, organizationID, knowledgeBaseID, parentID)
	if errors.Is(err, ErrGroupNotFound) {
		return nil, ErrGroupInvalid
	}
	if err != nil {
		return nil, err
	}
	if parent.IsDefault || parent.ParentID != nil {
		return nil, ErrGroupInvalid
	}
	return &parent.ID, nil
}

// nextGroupSortOrder 返回同级分组的下一个排序值。
func nextGroupSortOrder(ctx context.Context, db bun.IDB, knowledgeBaseID string, parentID *string) (int, error) {
	var maximum int
	query := db.NewSelect().TableExpr("knowledge_groups AS kg").
		ColumnExpr("COALESCE(MAX(kg.sort_order), 0)").
		Where("kg.knowledge_base_id = ?", knowledgeBaseID)
	if parentID == nil {
		query = query.Where("kg.parent_id IS NULL")
	} else {
		query = query.Where("kg.parent_id = ?", *parentID)
	}
	if err := query.Scan(ctx, &maximum); err != nil {
		return 0, err
	}
	return maximum + 1, nil
}
