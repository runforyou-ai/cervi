//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	roleaction "github.com/runforyou-ai/cervi/internal/actions/role"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateAgentAction 修改企业 AI 员工。
type UpdateAgentAction struct{ db *bun.DB }

// NewUpdateAgentAction 创建 AI 员工修改操作。
func NewUpdateAgentAction(db *bun.DB) *UpdateAgentAction { return &UpdateAgentAction{db: db} }

// Execute 在事务中保存 AI 员工基本资料和工作状态。
func (a *UpdateAgentAction) Execute(ctx context.Context, identity *servermodels.Identity, agentID string, input UpdateInput) (*Agent, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"displayName": ValidationDisplayNameRequired}}
	}
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
		_, err := roleaction.ValidateAssignment(ctx, tx, identity.Organization.ID, input.RoleID, domain.OrganizationIdentityTypeAgent)
		if errors.Is(err, roleaction.ErrAssignmentInvalid) || errors.Is(err, roleaction.ErrAgentAdministrator) {
			return &common.FieldError{Fields: map[string]common.FieldCode{"roleId": ValidationRoleInvalid}}
		}
		if err != nil {
			return err
		}
		teamIDs, _, err := validateAndLoadTeams(ctx, tx, identity.Organization.ID, input.TeamIDs)
		if err != nil {
			return err
		}
		storedAgent := &servermodels.Agent{}
		err = tx.NewSelect().Model(storedAgent).
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
			Set("display_name = ?", input.DisplayName).
			Set("role_id = ?", input.RoleID).
			Set("work_status_updated_at = CASE WHEN work_status <> ? THEN now() ELSE work_status_updated_at END", input.WorkStatus).
			Set("work_status = ?", input.WorkStatus).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", storedAgent.IdentityID).
			Where("type = ?", domain.OrganizationIdentityTypeAgent).
			Exec(ctx)
		if err != nil {
			return err
		}
		if err := teamaction.ReplaceIdentityTeams(ctx, tx, identity, storedAgent.IdentityID, teamIDs); err != nil {
			return err
		}
		output, err = loadAgent(ctx, tx, identity.Organization.ID, agentID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	return output, nil
}
