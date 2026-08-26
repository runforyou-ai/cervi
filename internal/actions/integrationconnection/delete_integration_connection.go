//go:build server

package integrationconnection

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteIntegrationConnectionAction 删除外部系统连接器。
type DeleteIntegrationConnectionAction struct {
	db *bun.DB
}

// NewDeleteIntegrationConnectionAction 创建连接器删除操作。
func NewDeleteIntegrationConnectionAction(db *bun.DB) *DeleteIntegrationConnectionAction {
	return &DeleteIntegrationConnectionAction{db: db}
}

// Execute 删除未被业务数据使用的连接器。
func (a *DeleteIntegrationConnectionAction) Execute(ctx context.Context, identity *servermodels.Identity, connectionID string) error {
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		connection, err := loadConnection(ctx, tx, identity.Organization.ID, connectionID, true)
		if err != nil {
			return err
		}
		inUse, err := connectionInUse(ctx, tx, identity.Organization.ID, connection.ID)
		if err != nil {
			return err
		}
		if inUse {
			return ErrInUse
		}
		_, err = tx.NewDelete().
			Model(connection).
			Where("organization_id = ?", identity.Organization.ID).
			WherePK().
			Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("delete integration connection: %w", err)
	}
	return nil
}
