//go:build server

package user

import (
	"context"
	"fmt"

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

// Execute 校验并保存当前用户的偏好设置。
func (a *UpdatePreferencesAction) Execute(ctx context.Context, identity *servermodels.Identity, input PreferencesInput) (*servermodels.Identity, error) {
	fields := validatePreferencesInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if err := validateIdentity(ctx, a.db, identity); err != nil {
		return nil, err
	}

	_, err := a.db.NewUpdate().
		Model((*servermodels.User)(nil)).
		Set("locale = ?", input.Locale).
		Set("time_zone = ?", input.TimeZone).
		Set("message_notifications_enabled = ?", input.MessageNotificationsEnabled).
		Set("updated_at = now()").
		Where("u.id = ?", identity.User.ID).
		Where("u.organization_id = ?", identity.Organization.ID).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update user preferences: %w", err)
	}
	updatedIdentity, err := loadCurrentIdentity(ctx, a.db, identity.Organization, identity.User.ID)
	if err != nil {
		return nil, fmt.Errorf("reload user preferences: %w", err)
	}
	return updatedIdentity, nil
}
