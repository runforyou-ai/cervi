//go:build server

package businesssystem

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateBusinessSystemAction 修改业务系统。
type UpdateBusinessSystemAction struct {
	db *bun.DB
}

// NewUpdateBusinessSystemAction 创建业务系统修改操作。
func NewUpdateBusinessSystemAction(db *bun.DB) *UpdateBusinessSystemAction {
	return &UpdateBusinessSystemAction{db: db}
}

// Execute 修改当前企业中的业务系统。
func (a *UpdateBusinessSystemAction) Execute(ctx context.Context, identity *servermodels.Identity, businessSystemID string, input Input) (*Record, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var businessSystem *servermodels.BusinessSystem
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		current, err := loadBusinessSystem(ctx, tx, identity.Organization.ID, businessSystemID, true)
		if err != nil {
			return err
		}
		current.Name = input.Name
		current.Description = input.Description
		current.URL = input.URL
		current.Enabled = input.Enabled
		if _, err := tx.NewUpdate().
			Model(current).
			Column("name", "description", "url", "enabled").
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			WherePK().
			Returning("*").
			Exec(ctx); err != nil {
			return err
		}
		businessSystem = current
		return nil
	})
	if isNameConflict(err) {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"name": ValidationNameDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("update business system: %w", err)
	}
	output := recordFromModel(*businessSystem)
	return &output, nil
}
