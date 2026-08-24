//go:build server

package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateStatusAction 修改用户账号状态。
type UpdateStatusAction struct{ db *bun.DB }

// NewUpdateStatusAction 创建用户账号状态修改操作。
func NewUpdateStatusAction(db *bun.DB) *UpdateStatusAction { return &UpdateStatusAction{db: db} }

// Execute 禁用或恢复用户账号，并在禁用时清理渠道分配。
func (a *UpdateStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, userID string, status domain.UserStatus) (*User, error) {
	if !common.ValidUUID(userID) {
		return nil, ErrNotFound
	}
	if status != domain.UserStatusActive && status != domain.UserStatusInactive {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"status": ValidationStatusInvalid}}
	}
	var output *User
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		administratorRoleID, err := lockAdministratorRole(ctx, tx, identity.Organization.ID)
		if err != nil {
			return err
		}
		updatedUser := &servermodels.User{}
		err = tx.NewUpdate().Model(updatedUser).
			Set("status = ?", status).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", userID).
			Returning("identity_id").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == domain.UserStatusInactive {
			if _, err := tx.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).
				Set("work_status = ?", domain.WorkStatusOffDuty).
				Set("work_status_updated_at = now()").
				Set("updated_at = now()").
				Where("organization_id = ?", identity.Organization.ID).
				Where("id = ?", updatedUser.IdentityID).
				Where("type = ?", domain.OrganizationIdentityTypeUser).
				Exec(ctx); err != nil {
				return err
			}
			if err := channelaction.ResetRoutingTarget(ctx, tx, identity.Organization.ID, domain.ChannelRoutingTargetTypeMember, updatedUser.IdentityID); err != nil {
				return err
			}
		}
		if err := ensureActiveAdministratorRemains(ctx, tx, identity.Organization.ID, administratorRoleID); err != nil {
			return err
		}
		output, err = loadUser(ctx, tx, identity.Organization.ID, userID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update user status: %w", err)
	}
	return output, nil
}
