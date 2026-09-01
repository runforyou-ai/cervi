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

// CreateKnowledgeBaseAction 创建企业知识库。
type CreateKnowledgeBaseAction struct {
	db *bun.DB
}

// NewCreateKnowledgeBaseAction 创建知识库新增操作。
func NewCreateKnowledgeBaseAction(db *bun.DB) *CreateKnowledgeBaseAction {
	return &CreateKnowledgeBaseAction{db: db}
}

// Execute 校验并创建当前企业的知识库。
func (a *CreateKnowledgeBaseAction) Execute(ctx context.Context, identity *servermodels.Identity, input Input) (*Record, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &common.FieldError{Fields: fields}
	}
	var record Record
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if err := validateDifyConnection(ctx, tx, identity.Organization.ID, input.IntegrationConnectionID); err != nil {
			return err
		}
		knowledgeBase := &servermodels.KnowledgeBase{
			OrganizationID: identity.Organization.ID, CreatedByUserID: identity.User.ID,
			Name: input.Name, Category: string(input.Category), Description: input.Description,
			IntegrationConnectionID: common.OptionalString(input.IntegrationConnectionID),
			ExternalResourceID:      common.OptionalString(input.ExternalResourceID),
		}
		_, err := tx.NewInsert().Model(knowledgeBase).
			Column("organization_id", "created_by_user_id", "name", "category", "description", "integration_connection_id", "external_resource_id").
			Returning("id, name, category, description, integration_connection_id, external_resource_id, created_at, updated_at").
			Exec(ctx)
		if isConstraintConflict(err, "knowledge_bases_organization_name_unique") {
			return &common.FieldError{Fields: map[string]common.FieldCode{"name": ValidationNameDuplicate}}
		}
		if isConstraintConflict(err, "knowledge_bases_external_resource_unique") {
			return &common.FieldError{Fields: map[string]common.FieldCode{"externalResourceId": ValidationExternalResourceDuplicate}}
		}
		if err != nil {
			return err
		}
		defaultGroup := &servermodels.KnowledgeGroup{KnowledgeBaseID: knowledgeBase.ID, Name: "", IsDefault: true, SortOrder: 0}
		if _, err := tx.NewInsert().Model(defaultGroup).
			Column("knowledge_base_id", "name", "is_default", "sort_order").
			Exec(ctx); err != nil {
			return err
		}
		record = recordFromModel(*knowledgeBase)
		record.Groups, err = loadGroupRecords(ctx, tx, knowledgeBase.ID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create knowledge base: %w", err)
	}
	return &record, nil
}
