//go:build server

package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// validateIdentity 校验当前用户仍是企业的有效成员。
func validateIdentity(ctx context.Context, db bun.IDB, identity *servermodels.Identity) error {
	if identity == nil || !common.ValidUUID(identity.Organization.ID) || !common.ValidUUID(identity.User.ID) || identity.User.OrganizationID != identity.Organization.ID {
		return common.ErrIdentityInvalid
	}
	exists, err := db.NewSelect().Model((*servermodels.User)(nil)).
		Where("organization_id = ?", identity.Organization.ID).
		Where("id = ?", identity.User.ID).
		Where("status = 'active'").
		Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return common.ErrIdentityInvalid
	}
	return nil
}

// loadDirectoryUser 读取企业成员及所属团队。
func loadDirectoryUser(ctx context.Context, db bun.IDB, organizationID, userID string) (*DirectoryUser, error) {
	if !common.ValidUUID(userID) {
		return nil, ErrNotFound
	}
	user := &DirectoryUser{}
	err := db.NewSelect().TableExpr("users AS u").
		ColumnExpr("u.id::text AS id").
		Column("email", "display_name", "role", "status", "work_status", "created_at").
		Where("u.id = ?", userID).
		Where("u.organization_id = ?", organizationID).
		Scan(ctx, user)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.Teams, err = loadUserTeams(ctx, db, organizationID, userID)
	return user, err
}

// loadUserTeams 读取成员所属的全部团队。
func loadUserTeams(ctx context.Context, db bun.IDB, organizationID, userID string) ([]TeamSummary, error) {
	teams := make([]TeamSummary, 0)
	err := db.NewSelect().TableExpr("team_members AS tm").
		ColumnExpr("t.id::text AS id, t.name").
		Join("JOIN teams AS t ON t.id = tm.team_id AND t.organization_id = tm.organization_id").
		Where("tm.organization_id = ?", organizationID).
		Where("tm.identity_type = ?", domain.MemberIdentityTypeUser).
		Where("tm.identity_id = ?", userID).
		OrderExpr("lower(t.name) ASC, t.id ASC").
		Scan(ctx, &teams)
	return teams, err
}

// validateTeamIDs 校验团队列表全部属于当前企业并去重。
func validateTeamIDs(ctx context.Context, db bun.IDB, organizationID string, teamIDs []string) ([]string, error) {
	unique := make(map[string]struct{}, len(teamIDs))
	for _, teamID := range teamIDs {
		if !common.ValidUUID(teamID) {
			return nil, &ValidationError{Fields: map[string]ValidationCode{"teamIds": ValidationTeamInvalid}}
		}
		unique[teamID] = struct{}{}
	}
	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return ids, nil
	}
	count, err := db.NewSelect().TableExpr("teams AS t").
		Where("t.organization_id = ?", organizationID).
		Where("t.id IN (?)", bun.In(ids)).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	if count != len(ids) {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"teamIds": ValidationTeamInvalid}}
	}
	return ids, nil
}

// replaceUserTeams 按差集修改成员所属团队。
func replaceUserTeams(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, userID string, teamIDs []string) error {
	ids, err := validateTeamIDs(ctx, tx, identity.Organization.ID, teamIDs)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		_, err = tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("identity_type = ?", domain.MemberIdentityTypeUser).
			Where("identity_id = ?", userID).
			Exec(ctx)
		return err
	}
	if _, err := tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
		Where("organization_id = ?", identity.Organization.ID).
		Where("identity_type = ?", domain.MemberIdentityTypeUser).
		Where("identity_id = ?", userID).
		Where("team_id NOT IN (?)", bun.In(ids)).
		Exec(ctx); err != nil {
		return err
	}
	relations := make([]servermodels.TeamMember, 0, len(ids))
	for _, teamID := range ids {
		relations = append(relations, servermodels.TeamMember{OrganizationID: identity.Organization.ID, TeamID: teamID, IdentityType: string(domain.MemberIdentityTypeUser), IdentityID: userID, CreatedByUserID: identity.User.ID})
	}
	if _, err := tx.NewInsert().Model(&relations).
		Column("organization_id", "team_id", "identity_type", "identity_id", "created_by_user_id").
		On("CONFLICT (organization_id, team_id, identity_type, identity_id) DO NOTHING").
		Exec(ctx); err != nil {
		return fmt.Errorf("insert user teams: %w", err)
	}
	return nil
}
