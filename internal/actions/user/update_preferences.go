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
	input, fields := normalizePreferencesInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return nil, common.ErrIdentityInvalid
	}

	user := &servermodels.User{}
	result, err := a.db.NewUpdate().
		Model(user).
		Set("locale = ?", input.Locale).
		Set("time_zone = ?", input.TimeZone).
		Set("updated_at = now()").
		Where("u.id = ?", identity.User.ID).
		Where("u.organization_id = ?", identity.Organization.ID).
		Returning("id, organization_id, email, display_name, role, status, locale, time_zone").
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
	return user, nil
}
