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

// UpdateKnowledgeBaseAction 修改企业知识库。
type UpdateKnowledgeBaseAction struct {
	db *bun.DB
}

// NewUpdateKnowledgeBaseAction 创建知识库修改操作。
func NewUpdateKnowledgeBaseAction(db *bun.DB) *UpdateKnowledgeBaseAction {
	return &UpdateKnowledgeBaseAction{db: db}
}

// Execute 校验并修改当前企业的知识库。
func (a *UpdateKnowledgeBaseAction) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID string, input Input) (*Record, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &common.FieldError{Fields: fields}
	}
	if !common.ValidUUID(knowledgeBaseID) {
		return nil, ErrNotFound
	}
	var record Record
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if err := validateDifyConnection(ctx, tx, identity.Organization.ID, input.IntegrationConnectionID); err != nil {
			return err
		}
		result, err := tx.NewUpdate().Model((*servermodels.KnowledgeBase)(nil)).
			Set("name = ?", input.Name).
			Set("category = ?", input.Category).
			Set("description = ?", input.Description).
			Set("integration_connection_id = ?", common.OptionalString(input.IntegrationConnectionID)).
			Set("external_resource_id = ?", common.OptionalString(input.ExternalResourceID)).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", knowledgeBaseID).
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
		if err := rowsAffectedOne(result, ErrNotFound); err != nil {
			return err
		}
		updated, err := loadKnowledgeBaseRecord(ctx, tx, identity.Organization.ID, knowledgeBaseID)
		if err != nil {
			return err
		}
		record = *updated
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update knowledge base: %w", err)
	}
	return &record, nil
}
