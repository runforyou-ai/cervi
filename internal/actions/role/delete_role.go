//go:build server

package role

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteRoleAction 删除自定义角色。
type DeleteRoleAction struct {
	db *bun.DB
}

// NewDeleteRoleAction 创建角色删除操作。
func NewDeleteRoleAction(db *bun.DB) *DeleteRoleAction {
	return &DeleteRoleAction{db: db}
}

// Execute 删除当前企业中的自定义角色及其权限。
func (a *DeleteRoleAction) Execute(ctx context.Context, identity *servermodels.Identity, roleID string) error {
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		role, err := loadRole(ctx, tx, identity.Organization.ID, roleID, true)
		if err != nil {
			return err
		}
		if domain.RoleKind(role.Kind) != domain.RoleKindCustom {
			return ErrBuiltInDeleteForbidden
		}
		inUse, err := tx.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("role_id = ?", role.ID).
			Exists(ctx)
		if err != nil {
			return err
		}
		if inUse {
			return ErrInUse
		}
		if _, err := tx.NewDelete().
			Model((*servermodels.RolePermission)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("role_id = ?", role.ID).
			Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewDelete().
			Model(role).
			Where("organization_id = ?", identity.Organization.ID).
			WherePK().
			Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}
