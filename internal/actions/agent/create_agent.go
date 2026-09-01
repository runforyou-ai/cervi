//go:build server

// Package agent 实现企业 AI 员工操作。
package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"uuid"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	roleaction "github.com/runforyou-ai/cervi/internal/actions/role"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateAgentAction 创建企业 AI 员工。
type CreateAgentAction struct{ db *bun.DB }

// NewCreateAgentAction 创建 AI 员工新增操作。
func NewCreateAgentAction(db *bun.DB) *CreateAgentAction {
	return &CreateAgentAction{db: db}
}

// Execute 创建 AI 员工、当前执行配置和团队关系。
func (a *CreateAgentAction) Execute(ctx context.Context, identity *servermodels.Identity, input CreateInput) (*Agent, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"displayName": ValidationDisplayNameRequired}}
	}
	executionInput, err := normalizeExecutionInput(input.Execution)
	if err != nil {
		return nil, err
	}
	var output *Agent
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		role, err := roleaction.ValidateAssignment(ctx, tx, identity.Organization.ID, input.RoleID, domain.OrganizationIdentityTypeAgent)
		if errors.Is(err, roleaction.ErrAssignmentInvalid) || errors.Is(err, roleaction.ErrAgentAdministrator) {
			return &common.FieldError{Fields: map[string]common.FieldCode{"roleId": ValidationRoleInvalid}}
		}
		if err != nil {
			return err
		}
		teamIDs, teams, err := validateAndLoadTeams(ctx, tx, identity.Organization.ID, input.TeamIDs)
		if err != nil {
			return err
		}
		model, err := loadManagedExecutionModel(ctx, tx, identity.Organization.ID, *executionInput.Managed)
		if err != nil {
			return err
		}
		revisionID := uuid.NewV7()
		organizationIdentity := &servermodels.OrganizationIdentity{
			OrganizationID: identity.Organization.ID,
			Type:           string(domain.OrganizationIdentityTypeAgent),
			RoleID:         input.RoleID,
			DisplayName:    input.DisplayName,
			WorkStatus:     string(domain.WorkStatusWorking),
		}
		if _, err := tx.NewInsert().Model(organizationIdentity).
			Column("organization_id", "type", "role_id", "display_name", "work_status").
			Returning("id, created_at").
			Exec(ctx); err != nil {
			return err
		}
		agent := &servermodels.Agent{
			IdentityID:       organizationIdentity.ID,
			OrganizationID:   identity.Organization.ID,
			ActiveRevisionID: revisionID.String(),
			Status:           string(domain.UserStatusActive),
		}
		if _, err := tx.NewInsert().Model(agent).
			Column("identity_id", "organization_id", "active_revision_id", "status").
			Returning("id").
			Exec(ctx); err != nil {
			return err
		}
		execution, err := insertExecutionRevision(ctx, tx, identity, agent.ID, revisionID.String(), executionInput, model)
		if err != nil {
			return err
		}
		if len(teamIDs) > 0 {
			relations := make([]servermodels.TeamMember, 0, len(teamIDs))
			for _, teamID := range teamIDs {
				relations = append(relations, servermodels.TeamMember{
					OrganizationID:  identity.Organization.ID,
					TeamID:          teamID,
					IdentityID:      organizationIdentity.ID,
					CreatedByUserID: identity.User.ID,
				})
			}
			if _, err := tx.NewInsert().Model(&relations).
				Column("organization_id", "team_id", "identity_id", "created_by_user_id").
				Exec(ctx); err != nil {
				return err
			}
		}
		output = &Agent{ID: agent.ID, IdentityID: organizationIdentity.ID, DisplayName: organizationIdentity.DisplayName, RoleID: role.ID, RoleKind: domain.RoleKind(role.Kind), RoleName: role.Name, Status: domain.UserStatus(agent.Status), WorkStatus: domain.WorkStatus(organizationIdentity.WorkStatus), Teams: teams, Execution: execution, CreatedAt: organizationIdentity.CreatedAt}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	return output, nil
}

// validateAndLoadTeams 规范化团队编号并锁定当前企业中的团队。
func validateAndLoadTeams(ctx context.Context, db bun.IDB, organizationID string, values []string) ([]string, []TeamSummary, error) {
	teamIDs, valid := common.NormalizeUUIDs(values)
	if !valid {
		return nil, nil, &common.FieldError{Fields: map[string]common.FieldCode{"teamIds": ValidationTeamInvalid}}
	}
	if len(teamIDs) == 0 {
		return teamIDs, []TeamSummary{}, nil
	}
	teams := make([]TeamSummary, 0, len(teamIDs))
	if err := db.NewSelect().TableExpr("teams AS t").
		ColumnExpr("t.id::text, t.name").
		Where("t.organization_id = ?", organizationID).
		Where("t.id IN (?)", bun.In(teamIDs)).
		OrderExpr("lower(t.name) ASC, t.id ASC").
		For("KEY SHARE").
		Scan(ctx, &teams); err != nil {
		return nil, nil, err
	}
	if len(teams) != len(teamIDs) {
		return nil, nil, &common.FieldError{Fields: map[string]common.FieldCode{"teamIds": ValidationTeamInvalid}}
	}
	return teamIDs, teams, nil
}
