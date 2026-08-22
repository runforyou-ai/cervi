//go:build server

package user

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdatePreferencesAction 修改当前用户的语言和时区设置。
type UpdatePreferencesAction struct {
	db *bun.DB
}

// NewUpdatePreferencesAction 创建语言和时区修改操作。
func NewUpdatePreferencesAction(db *bun.DB) *UpdatePreferencesAction {
	return &UpdatePreferencesAction{db: db}
}

// Execute 校验并保存当前用户的语言和时区设置。
func (a *UpdatePreferencesAction) Execute(ctx context.Context, identity *servermodels.Identity, input PreferencesInput) (*servermodels.User, error) {
	fields := validatePreferencesInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return nil, common.ErrIdentityInvalid
	}

	result, err := a.db.NewUpdate().
		Model((*servermodels.User)(nil)).
		Set("locale = ?", input.Locale).
		Set("time_zone = ?", input.TimeZone).
		Set("updated_at = now()").
		Where("u.id = ?", identity.User.ID).
		Where("u.organization_id = ?", identity.Organization.ID).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update user preferences: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read updated user preferences count: %w", err)
	}
	if rows == 0 {
		return nil, common.ErrIdentityInvalid
	}
	user, err := loadUser(ctx, a.db, identity.Organization.ID, identity.User.ID)
	if err != nil {
		return nil, fmt.Errorf("reload user preferences: %w", err)
	}
	return user, nil
}
