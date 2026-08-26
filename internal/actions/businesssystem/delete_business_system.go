//go:build server

package businesssystem

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteBusinessSystemAction 删除业务系统。
type DeleteBusinessSystemAction struct {
	db *bun.DB
}

// NewDeleteBusinessSystemAction 创建业务系统删除操作。
func NewDeleteBusinessSystemAction(db *bun.DB) *DeleteBusinessSystemAction {
	return &DeleteBusinessSystemAction{db: db}
}

// Execute 删除当前企业中的业务系统。
func (a *DeleteBusinessSystemAction) Execute(ctx context.Context, identity *servermodels.Identity, businessSystemID string) error {
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		businessSystem, err := loadBusinessSystem(ctx, tx, identity.Organization.ID, businessSystemID, true)
		if err != nil {
			return err
		}
		_, err = tx.NewDelete().
			Model(businessSystem).
			Where("organization_id = ?", identity.Organization.ID).
			WherePK().
			Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("delete business system: %w", err)
	}
	return nil
}
