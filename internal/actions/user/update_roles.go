//go:build server

package user

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateRolesAction 在一个事务中批量调整企业成员角色。
type UpdateRolesAction struct{ db *bun.DB }

// NewUpdateRolesAction 创建企业成员角色批量调整操作。
func NewUpdateRolesAction(db *bun.DB) *UpdateRolesAction { return &UpdateRolesAction{db: db} }

// Execute 规范化并校验成员和角色后一次性完成调整。
func (a *UpdateRolesAction) Execute(ctx context.Context, identity *servermodels.Identity, changes []RoleChangeInput) error {
	userIDs := make([]string, 0, len(changes))
	roleIDs := make([]string, 0, len(changes))
	users := make(map[string]struct{}, len(changes))
	roles := make(map[string]struct{}, len(changes))
	for index := range changes {
		change := &changes[index]
		var userIDValid, roleIDValid bool
		change.UserID, userIDValid = common.NormalizeUUID(change.UserID)
		change.RoleID, roleIDValid = common.NormalizeUUID(change.RoleID)
		if !userIDValid || !roleIDValid {
			return ErrRoleChangesInvalid
		}
		if _, exists := users[change.UserID]; exists {
			return ErrRoleChangesInvalid
		}
		users[change.UserID] = struct{}{}
		userIDs = append(userIDs, change.UserID)
		if _, exists := roles[change.RoleID]; !exists {
			roles[change.RoleID] = struct{}{}
			roleIDs = append(roleIDs, change.RoleID)
		}
	}

	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		if len(changes) == 0 {
			return nil
		}
		administratorRoleID, err := lockAdministratorRole(ctx, tx, identity.Organization.ID)
		if err != nil {
			return err
		}
		var lockedRoles []servermodels.Role
		if err := tx.NewSelect().Model(&lockedRoles).
			Column("id").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id IN (?)", bun.In(roleIDs)).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			return err
		}
		if len(lockedRoles) != len(roleIDs) {
			return ErrRoleChangesInvalid
		}
		var lockedUsers []servermodels.User
		if err := tx.NewSelect().Model(&lockedUsers).
			Column("id").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id IN (?)", bun.In(userIDs)).
			For("UPDATE").
			Scan(ctx); err != nil {
			return err
		}
		if len(lockedUsers) != len(userIDs) {
			return ErrRoleChangesInvalid
		}
		for _, change := range changes {
			if _, err := tx.NewUpdate().Model((*servermodels.User)(nil)).
				Set("role_id = ?", change.RoleID).
				Set("updated_at = now()").
				Where("organization_id = ?", identity.Organization.ID).
				Where("id = ?", change.UserID).
				Exec(ctx); err != nil {
				return err
			}
		}
		return ensureActiveAdministratorRemains(ctx, tx, identity.Organization.ID, administratorRoleID)
	})
	if err != nil {
		return fmt.Errorf("update user roles: %w", err)
	}
	return nil
}
