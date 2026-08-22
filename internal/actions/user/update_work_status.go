//go:build server

package user

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
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
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return nil, common.ErrIdentityInvalid
	}

	result, err := a.db.NewUpdate().
		Model((*servermodels.User)(nil)).
		Set("work_status = ?", input.WorkStatus).
		Set("work_status_updated_at = now()").
		Set("updated_at = now()").
		Where("u.id = ?", identity.User.ID).
		Where("u.organization_id = ?", identity.Organization.ID).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update user work status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read updated user work status count: %w", err)
	}
	if rows == 0 {
		return nil, common.ErrIdentityInvalid
	}
	user, err := loadUser(ctx, a.db, identity.Organization.ID, identity.User.ID)
	if err != nil {
		return nil, fmt.Errorf("reload user work status: %w", err)
	}
	return user, nil
}
