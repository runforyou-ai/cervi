//go:build server

package integrationconnection

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateIntegrationConnectionAction 修改外部系统连接器。
type UpdateIntegrationConnectionAction struct {
	db *bun.DB
}

// NewUpdateIntegrationConnectionAction 创建连接器修改操作。
func NewUpdateIntegrationConnectionAction(db *bun.DB) *UpdateIntegrationConnectionAction {
	return &UpdateIntegrationConnectionAction{db: db}
}

// Execute 修改连接器名称、说明和配置。
func (a *UpdateIntegrationConnectionAction) Execute(ctx context.Context, identity *servermodels.Identity, connectionID string, input Input) (*Record, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var connection *servermodels.IntegrationConnection
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		current, err := loadConnection(ctx, tx, identity.Organization.ID, connectionID, true)
		if err != nil {
			return err
		}
		if current.Type != string(input.Type) {
			inUse, err := connectionInUse(ctx, tx, identity.Organization.ID, current.ID)
			if err != nil {
				return err
			}
			if inUse {
				return ErrInUse
			}
		}
		current.Type = string(input.Type)
		current.Name = input.Name
		current.Description = input.Description
		current.Configuration = servermodels.IntegrationConnectionConfiguration{
			APIURL: input.Configuration.APIURL,
			APIKey: input.Configuration.APIKey,
		}
		if _, err := tx.NewUpdate().
			Model(current).
			Column("connector_type", "name", "description", "configuration").
			Set("status = ?", domain.IntegrationConnectionStatusUntested).
			Set("last_tested_at = NULL").
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			WherePK().
			Returning("*").
			Exec(ctx); err != nil {
			return err
		}
		connection = current
		return nil
	})
	if isNameConflict(err) {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"name": ValidationNameDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("update integration connection: %w", err)
	}
	output := recordFromModel(*connection)
	return &output, nil
}
