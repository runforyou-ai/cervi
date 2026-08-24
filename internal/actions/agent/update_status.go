//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateStatusAction 修改 AI 员工状态。
type UpdateStatusAction struct{ db *bun.DB }

// NewUpdateStatusAction 创建 AI 员工状态修改操作。
func NewUpdateStatusAction(db *bun.DB) *UpdateStatusAction { return &UpdateStatusAction{db: db} }

// Execute 禁用或恢复 AI 员工账号，并在禁用时清理渠道分配。
func (a *UpdateStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, agentID string, status domain.UserStatus) (*Agent, error) {
	if !common.ValidUUID(agentID) {
		return nil, ErrNotFound
	}
	if status != domain.UserStatusActive && status != domain.UserStatusInactive {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"status": ValidationStatusInvalid}}
	}
	var output *Agent
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		updatedAgent := &servermodels.Agent{}
		err := tx.NewUpdate().Model(updatedAgent).
			Set("status = ?", status).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", agentID).
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
				Where("id = ?", updatedAgent.IdentityID).
				Where("type = ?", domain.OrganizationIdentityTypeAgent).
				Exec(ctx); err != nil {
				return err
			}
			if err := channelaction.ResetRoutingTarget(ctx, tx, identity.Organization.ID, domain.ChannelRoutingTargetTypeMember, updatedAgent.IdentityID); err != nil {
				return err
			}
		}
		output, err = loadAgent(ctx, tx, identity.Organization.ID, agentID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update agent status: %w", err)
	}
	return output, nil
}
