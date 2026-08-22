//go:build server

package user

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateStatusAction 修改企业成员账号状态。
type UpdateStatusAction struct{ db *bun.DB }

// NewUpdateStatusAction 创建企业成员状态修改操作。
func NewUpdateStatusAction(db *bun.DB) *UpdateStatusAction { return &UpdateStatusAction{db: db} }

// Execute 停用或恢复企业成员账号。
func (a *UpdateStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, userID string, status domain.UserStatus) (*DirectoryUser, error) {
	if !common.ValidUUID(userID) {
		return nil, ErrNotFound
	}
	if status != domain.UserStatusActive && status != domain.UserStatusInactive {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"status": ValidationStatusInvalid}}
	}
	var output *DirectoryUser
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		administratorRoleID, err := lockAdministratorRole(ctx, tx, identity.Organization.ID)
		if err != nil {
			return err
		}
		if status == domain.UserStatusInactive && userID == identity.User.ID {
			return ErrSelfDeactivate
		}
		result, err := tx.NewUpdate().Model((*servermodels.User)(nil)).
			Set("status = ?", status).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", userID).
			Exec(ctx)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrNotFound
		}
		if err := ensureActiveAdministratorRemains(ctx, tx, identity.Organization.ID, administratorRoleID); err != nil {
			return err
		}
		output, err = loadDirectoryUser(ctx, tx, identity.Organization.ID, userID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update user status: %w", err)
	}
	return output, nil
}
