//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateInput 定义 AI 员工可编辑字段。
type UpdateInput struct {
	DisplayName string
	TeamIDs     []string
}

// UpdateAgentAction 修改企业 AI 员工。
type UpdateAgentAction struct{ db *bun.DB }

// NewUpdateAgentAction 创建 AI 员工修改操作。
func NewUpdateAgentAction(db *bun.DB) *UpdateAgentAction { return &UpdateAgentAction{db: db} }

// Execute 修改 AI 员工名称和所属团队。
func (a *UpdateAgentAction) Execute(ctx context.Context, identity *servermodels.Identity, agentID string, input UpdateInput) (*DirectoryAgent, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"displayName": ValidationDisplayNameRequired}}
	}
	if !common.ValidUUID(agentID) {
		return nil, ErrNotFound
	}
	var output *DirectoryAgent
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		teamIDs, _, err := validateAndLoadTeams(ctx, tx, identity.Organization.ID, input.TeamIDs)
		if err != nil {
			return err
		}
		storedAgent := &servermodels.Agent{}
		err = tx.NewSelect().Model(storedAgent).
			Column("identity_id").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", agentID).
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).
			Set("display_name = ?", input.DisplayName).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", storedAgent.IdentityID).
			Where("type = ?", domain.OrganizationIdentityTypeAgent).
			Exec(ctx)
		if err != nil {
			return err
		}
		if err := replaceAgentTeams(ctx, tx, identity, storedAgent.IdentityID, teamIDs); err != nil {
			return err
		}
		output, err = loadDirectoryAgent(ctx, tx, identity.Organization.ID, agentID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	return output, nil
}

// replaceAgentTeams 按差集修改 AI 员工所属团队。
func replaceAgentTeams(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, organizationIdentityID string, teamIDs []string) error {
	if len(teamIDs) == 0 {
		_, err := tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("identity_id = ?", organizationIdentityID).
			Exec(ctx)
		return err
	}
	if _, err := tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
		Where("organization_id = ?", identity.Organization.ID).
		Where("identity_id = ?", organizationIdentityID).
		Where("team_id NOT IN (?)", bun.In(teamIDs)).
		Exec(ctx); err != nil {
		return err
	}
	relations := make([]servermodels.TeamMember, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		relations = append(relations, servermodels.TeamMember{OrganizationID: identity.Organization.ID, TeamID: teamID, IdentityID: organizationIdentityID, CreatedByUserID: identity.User.ID})
	}
	if _, err := tx.NewInsert().Model(&relations).
		Column("organization_id", "team_id", "identity_id", "created_by_user_id").
		On("CONFLICT (organization_id, team_id, identity_id) DO NOTHING").
		Exec(ctx); err != nil {
		return fmt.Errorf("insert agent teams: %w", err)
	}
	return nil
}
