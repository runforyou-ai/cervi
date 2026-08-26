//go:build server

package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
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
func (a *UpdateWorkStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, input WorkStatusInput) (*servermodels.Identity, error) {
	fields := validateWorkStatusInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var updatedIdentity *servermodels.Identity
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		storedUser := &servermodels.User{}
		err := tx.NewSelect().Model(storedUser).
			Column("identity_id").
			Where("id = ?", identity.User.ID).
			Where("identity_id = ?", identity.User.IdentityID).
			Where("organization_id = ?", identity.Organization.ID).
			Where("status = ?", domain.UserStatusActive).
			For("UPDATE").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrIdentityInvalid
		}
		if err != nil {
			return err
		}
		if _, err := tx.NewUpdate().
			Model((*servermodels.OrganizationIdentity)(nil)).
			Set("work_status = ?", input.WorkStatus).
			Set("work_status_updated_at = now()").
			Set("updated_at = now()").
			Where("oi.id = ?", storedUser.IdentityID).
			Where("oi.organization_id = ?", identity.Organization.ID).
			Where("oi.type = ?", domain.OrganizationIdentityTypeUser).
			Exec(ctx); err != nil {
			return err
		}
		updatedIdentity, err = loadCurrentIdentity(ctx, tx, identity.Organization, identity.User.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update user work status: %w", err)
	}
	return updatedIdentity, nil
}
