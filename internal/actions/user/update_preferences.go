//go:build server

package user

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdatePreferencesAction 修改当前用户的偏好设置。
type UpdatePreferencesAction struct {
	db *bun.DB
}

// NewUpdatePreferencesAction 创建用户偏好修改操作。
func NewUpdatePreferencesAction(db *bun.DB) *UpdatePreferencesAction {
	return &UpdatePreferencesAction{db: db}
}

// Execute 在事务内校验并保存当前用户的偏好设置。
func (a *UpdatePreferencesAction) Execute(ctx context.Context, identity *servermodels.Identity, input PreferencesInput) (*servermodels.Identity, error) {
	fields := validatePreferencesInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var updatedIdentity *servermodels.Identity
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().
			Model((*servermodels.User)(nil)).
			Set("locale = ?", input.Locale).
			Set("time_zone = ?", input.TimeZone).
			Set("message_notifications_enabled = ?", input.MessageNotificationsEnabled).
			Set("updated_at = now()").
			Where("u.id = ?", identity.User.ID).
			Where("u.organization_id = ?", identity.Organization.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("update user preferences: %w", err)
		}
		reloaded, err := loadCurrentIdentity(ctx, tx, identity.Organization, identity.User.ID)
		if err != nil {
			return fmt.Errorf("reload user preferences: %w", err)
		}
		updatedIdentity = reloaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updatedIdentity, nil
}
