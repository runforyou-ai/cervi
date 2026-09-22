//go:build server

package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/actions/serviceassignment"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// UpdateWorkStatusAction 修改当前用户主动设置的工作状态。
type UpdateWorkStatusAction struct {
	db       *bun.DB
	enqueuer servertask.TxEnqueuer
}

// NewUpdateWorkStatusAction 创建工作状态修改操作。
func NewUpdateWorkStatusAction(db *bun.DB, enqueuer servertask.TxEnqueuer) *UpdateWorkStatusAction {
	return &UpdateWorkStatusAction{db: db, enqueuer: enqueuer}
}

// Execute 校验并保存当前用户的工作状态，切换为工作中时从所在队列补分配。
func (a *UpdateWorkStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, input WorkStatusInput) (*servermodels.Identity, error) {
	// 校验工作状态。
	fields := make(map[string]ValidationCode)
	if input.WorkStatus != domain.WorkStatusWorking &&
		input.WorkStatus != domain.WorkStatusAway &&
		input.WorkStatus != domain.WorkStatusOffDuty {
		fields["workStatus"] = ValidationWorkStatusInvalid
	}
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var updatedIdentity *servermodels.Identity
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
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
		var previousStatus domain.WorkStatus
		if err := tx.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).Column("oi.work_status").
			Where("oi.organization_id = ? AND oi.id = ?", identity.Organization.ID, storedUser.IdentityID).
			Scan(ctx, &previousStatus); err != nil {
			return err
		}
		if _, err := identityaction.UpdateUserIdentity(ctx, tx, identity.Organization.ID, storedUser.IdentityID, tx.NewUpdate().
			Model((*servermodels.OrganizationIdentity)(nil)).
			Set("work_status = ?", input.WorkStatus).
			Set("work_status_updated_at = now()").
			Set("updated_at = now()")); err != nil {
			return err
		}
		// 工作状态只在单聊页头展示，通知对端重读摘要即可。
		if err := chatstate.NotifyDirectPeersWorkStatusChanged(ctx, tx, identity.Organization.ID, storedUser.IdentityID); err != nil {
			return err
		}
		if input.WorkStatus == domain.WorkStatusWorking && previousStatus != domain.WorkStatusWorking {
			if err := serviceassignment.EnqueueBackfill(ctx, tx, a.enqueuer, serviceassignment.BackfillInput{OrganizationID: identity.Organization.ID, IdentityID: storedUser.IdentityID}); err != nil {
				return err
			}
		}
		updatedIdentity, err = loadCurrentIdentity(ctx, tx, identity.Organization, identity.User.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update user work status: %w", err)
	}
	return updatedIdentity, nil
}
