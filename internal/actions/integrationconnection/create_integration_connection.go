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

// CreateIntegrationConnectionAction 创建外部系统连接器。
type CreateIntegrationConnectionAction struct {
	db *bun.DB
}

// NewCreateIntegrationConnectionAction 创建连接器操作。
func NewCreateIntegrationConnectionAction(db *bun.DB) *CreateIntegrationConnectionAction {
	return &CreateIntegrationConnectionAction{db: db}
}

// Execute 创建连接器并保存类型化配置。
func (a *CreateIntegrationConnectionAction) Execute(ctx context.Context, identity *servermodels.Identity, input Input) (*Record, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	connection := servermodels.IntegrationConnection{
		OrganizationID: identity.Organization.ID,
		Type:           string(input.Type), Name: input.Name, Description: input.Description,
		Configuration: servermodels.IntegrationConnectionConfiguration{
			APIURL: input.Configuration.APIURL,
			APIKey: input.Configuration.APIKey,
		},
		Status: string(domain.IntegrationConnectionStatusUntested),
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		_, err := tx.NewInsert().
			Model(&connection).
			Column("organization_id", "connector_type", "name", "description", "configuration", "status").
			Returning("*").
			Exec(ctx)
		return err
	})
	if isNameConflict(err) {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"name": ValidationNameDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("create integration connection: %w", err)
	}
	output := recordFromModel(connection)
	return &output, nil
}
