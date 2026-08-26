//go:build server

package businesssystem

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateBusinessSystemAction 创建业务系统。
type CreateBusinessSystemAction struct {
	db *bun.DB
}

// NewCreateBusinessSystemAction 创建业务系统操作。
func NewCreateBusinessSystemAction(db *bun.DB) *CreateBusinessSystemAction {
	return &CreateBusinessSystemAction{db: db}
}

// Execute 在当前企业中创建业务系统。
func (a *CreateBusinessSystemAction) Execute(ctx context.Context, identity *servermodels.Identity, input Input) (*Record, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var businessSystem servermodels.BusinessSystem
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		businessSystem = servermodels.BusinessSystem{
			OrganizationID: identity.Organization.ID, Name: input.Name, Description: input.Description,
			URL: input.URL, Enabled: input.Enabled,
		}
		_, err := tx.NewInsert().
			Model(&businessSystem).
			Column("organization_id", "name", "description", "url", "enabled").
			Returning("*").
			Exec(ctx)
		return err
	})
	if isNameConflict(err) {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"name": ValidationNameDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("create business system: %w", err)
	}
	output := recordFromModel(businessSystem)
	return &output, nil
}
