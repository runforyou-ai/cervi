//go:build server

package user

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateWorkStatusAction 修改当前用户主动设置的工作状态。
type UpdateWorkStatusAction struct {
	db *bun.DB
}

// NewUpdateWorkStatusAction 创建工作状态修改操作。
func NewUpdateWorkStatusAction(db *bun.DB) *UpdateWorkStatusAction {
	return &UpdateWorkStatusAction{db: db}
}

// Execute 校验并保存当前用户的工作状态。
func (a *UpdateWorkStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, input WorkStatusInput) (*servermodels.User, error) {
	fields := validateWorkStatusInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if err := validateIdentity(ctx, a.db, identity); err != nil {
		return nil, err
	}

	_, err := a.db.NewUpdate().
		Model((*servermodels.OrganizationIdentity)(nil)).
		Set("work_status = ?", input.WorkStatus).
		Set("work_status_updated_at = now()").
		Set("updated_at = now()").
		Where("oi.id = ?", identity.User.IdentityID).
		Where("oi.organization_id = ?", identity.Organization.ID).
		Where("oi.type = ?", domain.OrganizationIdentityTypeUser).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update user work status: %w", err)
	}
	user, err := loadUser(ctx, a.db, identity.Organization.ID, identity.User.ID)
	if err != nil {
		return nil, fmt.Errorf("reload user work status: %w", err)
	}
	return user, nil
}
