//go:build server

package team

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Summary 定义企业身份所属团队的精简字段。
type Summary struct {
	ID   string `bun:"id"`
	Name string `bun:"name"`
}

// LoadIdentityTeams 读取一个企业身份所属的全部团队。
func LoadIdentityTeams(ctx context.Context, db bun.IDB, organizationID, identityID string) ([]Summary, error) {
	teams := make([]Summary, 0)
	err := db.NewSelect().TableExpr("team_members AS tm").
		ColumnExpr("t.id::text AS id, t.name").
		Join("JOIN teams AS t ON t.id = tm.team_id AND t.organization_id = tm.organization_id").
		Where("tm.organization_id = ?", organizationID).
		Where("tm.identity_id = ?", identityID).
		OrderExpr("lower(t.name) ASC, t.id ASC").
		Scan(ctx, &teams)
	return teams, err
}

// LoadTeamsByIdentity 一次查询多个企业身份所属团队，按身份编号分组返回。
func LoadTeamsByIdentity(ctx context.Context, db bun.IDB, organizationID string, identityIDs []string) (map[string][]Summary, error) {
	grouped := make(map[string][]Summary, len(identityIDs))
	for _, identityID := range identityIDs {
		grouped[identityID] = make([]Summary, 0)
	}
	if len(identityIDs) == 0 {
		return grouped, nil
	}
	rows := make([]struct {
		IdentityID string `bun:"identity_id"`
		ID         string `bun:"id"`
		Name       string `bun:"name"`
	}, 0)
	err := db.NewSelect().TableExpr("team_members AS tm").
		ColumnExpr("tm.identity_id::text AS identity_id, t.id::text AS id, t.name").
		Join("JOIN teams AS t ON t.id = tm.team_id AND t.organization_id = tm.organization_id").
		Where("tm.organization_id = ?", organizationID).
		Where("tm.identity_id IN (?)", bun.In(identityIDs)).
		OrderExpr("lower(t.name) ASC, t.id ASC").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		grouped[row.IdentityID] = append(grouped[row.IdentityID], Summary{ID: row.ID, Name: row.Name})
	}
	return grouped, nil
}

// ReplaceIdentityTeams 按差集替换企业身份的团队关系；teamIDs 必须已校验属于当前企业。
func ReplaceIdentityTeams(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, organizationIdentityID string, teamIDs []string) error {
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
		return fmt.Errorf("insert identity teams: %w", err)
	}
	return nil
}
