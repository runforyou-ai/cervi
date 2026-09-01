//go:build server

package agent

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

// UpdateWorkStatusAction 修改 AI 员工工作状态。
type UpdateWorkStatusAction struct{ db *bun.DB }

// NewUpdateWorkStatusAction 创建 AI 员工工作状态修改操作。
func NewUpdateWorkStatusAction(db *bun.DB) *UpdateWorkStatusAction {
	return &UpdateWorkStatusAction{db: db}
}

// Execute 校验并保存 AI 员工的工作状态。
func (a *UpdateWorkStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, agentID string, input WorkStatusInput) (*Agent, error) {
	if !common.ValidUUID(agentID) {
		return nil, ErrNotFound
	}
	if input.WorkStatus != domain.WorkStatusWorking && input.WorkStatus != domain.WorkStatusAway && input.WorkStatus != domain.WorkStatusOffDuty {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"workStatus": ValidationWorkStatusInvalid}}
	}

	var output *Agent
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		storedAgent := &servermodels.Agent{}
		err := tx.NewSelect().Model(storedAgent).
			Column("identity_id", "status").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", agentID).
			For("UPDATE").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if domain.UserStatus(storedAgent.Status) == domain.UserStatusInactive && input.WorkStatus != domain.WorkStatusOffDuty {
			return &common.FieldError{Fields: map[string]common.FieldCode{"workStatus": ValidationWorkStatusUnavailable}}
		}
		_, err = tx.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).
			Set("work_status = ?", input.WorkStatus).
			Set("work_status_updated_at = now()").
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", storedAgent.IdentityID).
			Where("type = ?", domain.OrganizationIdentityTypeAgent).
			Exec(ctx)
		if err != nil {
			return err
		}
		output, err = loadAgent(ctx, tx, identity.Organization.ID, agentID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update agent work status: %w", err)
	}
	return output, nil
}
