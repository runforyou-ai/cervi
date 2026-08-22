//go:build server

// Package agent 实现企业 AI 员工操作。
package agent

import (
	"context"
	"fmt"
	"strings"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateInput 定义新增 AI 员工字段。
type CreateInput struct {
	DisplayName string
	TeamIDs     []string
}

// TeamSummary 定义 AI 员工所属团队摘要。
type TeamSummary struct {
	ID   string `bun:"id"`
	Name string `bun:"name"`
}

// CreateAgentAction 创建企业 AI 员工。
type CreateAgentAction struct{ db *bun.DB }

// NewCreateAgentAction 创建 AI 员工新增操作。
func NewCreateAgentAction(db *bun.DB) *CreateAgentAction {
	return &CreateAgentAction{db: db}
}

// Execute 创建企业成员、AI 员工及其团队关系。
func (a *CreateAgentAction) Execute(ctx context.Context, identity *servermodels.Identity, input CreateInput) (*DirectoryAgent, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"displayName": ValidationDisplayNameRequired}}
	}
	var output *DirectoryAgent
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		teamIDs, teams, err := validateAndLoadTeams(ctx, tx, identity.Organization.ID, input.TeamIDs)
		if err != nil {
			return err
		}
		member := &servermodels.OrganizationMember{
			OrganizationID: identity.Organization.ID,
			Type:           string(domain.MemberIdentityTypeAgent),
			DisplayName:    input.DisplayName,
			Status:         string(domain.UserStatusActive),
		}
		if _, err := tx.NewInsert().Model(member).
			Column("organization_id", "type", "display_name", "status").
			Returning("id, created_at").
			Exec(ctx); err != nil {
			return err
		}
		agent := &servermodels.Agent{ID: member.ID, OrganizationID: identity.Organization.ID}
		if _, err := tx.NewInsert().Model(agent).
			Column("id", "organization_id").
			Exec(ctx); err != nil {
			return err
		}
		if len(teamIDs) > 0 {
			relations := make([]servermodels.TeamMember, 0, len(teamIDs))
			for _, teamID := range teamIDs {
				relations = append(relations, servermodels.TeamMember{
					OrganizationID:  identity.Organization.ID,
					TeamID:          teamID,
					MemberID:        member.ID,
					CreatedByUserID: identity.User.ID,
				})
			}
			if _, err := tx.NewInsert().Model(&relations).
				Column("organization_id", "team_id", "member_id", "created_by_user_id").
				Exec(ctx); err != nil {
				return err
			}
		}
		output = &DirectoryAgent{ID: member.ID, DisplayName: member.DisplayName, Status: domain.UserStatus(member.Status), Teams: teams, CreatedAt: member.CreatedAt}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	return output, nil
}

// validateAndLoadTeams 校验并锁定 AI 员工所属团队。
func validateAndLoadTeams(ctx context.Context, db bun.IDB, organizationID string, values []string) ([]string, []TeamSummary, error) {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		teamID := strings.TrimSpace(value)
		if !common.ValidUUID(teamID) {
			return nil, nil, &common.FieldError{Fields: map[string]common.FieldCode{"teamIds": ValidationTeamInvalid}}
		}
		unique[teamID] = struct{}{}
	}
	teamIDs := make([]string, 0, len(unique))
	for teamID := range unique {
		teamIDs = append(teamIDs, teamID)
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
