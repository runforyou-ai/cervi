//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ServiceSessionHandoff 在管理操作事务中把 AI 员工负责的开放客服周期交给人工，并在提交后中断被取消的模型调用。
type ServiceSessionHandoff interface {
	HandOffAgentServiceSessions(ctx context.Context, db bun.IDB, organizationID, agentIdentityID, operationID string) ([]string, error)
	CancelRunContexts([]string)
}

// UpdateStatusAction 修改 AI 员工状态。
type UpdateStatusAction struct {
	db      *bun.DB
	handoff ServiceSessionHandoff
}

// NewUpdateStatusAction 创建 AI 员工状态修改操作。
func NewUpdateStatusAction(db *bun.DB, handoff ServiceSessionHandoff) *UpdateStatusAction {
	return &UpdateStatusAction{db: db, handoff: handoff}
}

// Execute 禁用或恢复 AI 员工账号，禁用时清理渠道分配并把其负责的开放客服周期交给人工。
func (a *UpdateStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, agentID string, status domain.UserStatus) (*Agent, error) {
	if !common.ValidUUID(agentID) {
		return nil, ErrNotFound
	}
	if status != domain.UserStatusActive && status != domain.UserStatusInactive {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"status": ValidationStatusInvalid}}
	}
	var output *Agent
	var cancelledRunIDs []string
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
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
			// 先锁定身份再改渠道与会话，与以该身份为目标的渠道编辑、入站路由和转交串行。
			if err := lockAgentIdentity(ctx, tx, identity.Organization.ID, updatedAgent.IdentityID, nil); err != nil {
				return err
			}
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
			cancelledRunIDs, err = a.handoff.HandOffAgentServiceSessions(ctx, tx, identity.Organization.ID, updatedAgent.IdentityID, uuid.NewV7().String())
			if err != nil {
				return err
			}
		}
		output, err = loadAgent(ctx, tx, identity.Organization.ID, agentID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update agent status: %w", err)
	}
	a.handoff.CancelRunContexts(cancelledRunIDs)
	return output, nil
}

// lockAgentIdentity 对 AI 员工身份取 FOR UPDATE，roleKind 非空时返回锁定时的角色类型。
func lockAgentIdentity(ctx context.Context, db bun.IDB, organizationID, identityID string, roleKind *domain.RoleKind) error {
	var kind domain.RoleKind
	if err := db.NewSelect().TableExpr("organization_identities AS oi").
		ColumnExpr("r.kind").
		Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id").
		Where("oi.organization_id = ? AND oi.id = ? AND oi.type = ?", organizationID, identityID, domain.OrganizationIdentityTypeAgent).
		For("UPDATE OF oi").
		Scan(ctx, &kind); err != nil {
		return fmt.Errorf("lock agent identity: %w", err)
	}
	if roleKind != nil {
		*roleKind = kind
	}
	return nil
}
