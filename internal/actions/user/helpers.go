//go:build server

package user

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

// validateIdentity 校验当前用户仍是企业的有效成员。
func validateIdentity(ctx context.Context, db bun.IDB, identity *servermodels.Identity) error {
	return identityaction.Validate(ctx, db, identity)
}

// loadUser 读取用户账号及其企业成员字段。
func loadUser(ctx context.Context, db bun.IDB, organizationID, userID string) (*servermodels.User, error) {
	user := &servermodels.User{}
	err := db.NewSelect().TableExpr("users AS u").
		ColumnExpr("u.id::text, u.organization_id::text, u.email, u.password_hash, u.role_id::text, u.locale, u.time_zone, u.work_status").
		ColumnExpr("om.display_name, om.status, om.avatar_file_id::text").
		Join("JOIN organization_members AS om ON om.id = u.id AND om.organization_id = u.organization_id AND om.type = ?", domain.MemberIdentityTypeUser).
		Where("u.organization_id = ?", organizationID).
		Where("u.id = ?", userID).
		Scan(ctx, user)
	return user, err
}

// loadDirectoryUser 读取企业成员、角色和所属团队。
func loadDirectoryUser(ctx context.Context, db bun.IDB, organizationID, userID string) (*DirectoryUser, error) {
	if !common.ValidUUID(userID) {
		return nil, ErrNotFound
	}
	user := &DirectoryUser{}
	err := db.NewSelect().TableExpr("users AS u").
		ColumnExpr("u.id::text AS id").
		ColumnExpr("u.email, om.display_name, om.status, u.work_status, om.created_at").
		ColumnExpr("r.id::text AS role_id, r.kind AS role_kind, r.name AS role_name").
		Join("JOIN organization_members AS om ON om.id = u.id AND om.organization_id = u.organization_id AND om.type = ?", domain.MemberIdentityTypeUser).
		Join("JOIN roles AS r ON r.id = u.role_id AND r.organization_id = u.organization_id").
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

// validateRoleID 校验并锁定当前企业的角色。
func validateRoleID(ctx context.Context, db bun.IDB, organizationID, roleID string) error {
	if !common.ValidUUID(roleID) {
		return &ValidationError{Fields: map[string]ValidationCode{"roleId": ValidationRoleInvalid}}
	}
	role := &servermodels.Role{}
	err := db.NewSelect().Model(role).
		Column("id").
		Where("organization_id = ?", organizationID).
		Where("id = ?", roleID).
		For("KEY SHARE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return &ValidationError{Fields: map[string]ValidationCode{"roleId": ValidationRoleInvalid}}
	}
	if err != nil {
		return err
	}
	return nil
}

// lockAdministratorRole 锁定管理员角色以串行维护有效管理员数量。
func lockAdministratorRole(ctx context.Context, db bun.IDB, organizationID string) (string, error) {
	role := &servermodels.Role{}
	err := db.NewSelect().Model(role).
		Column("id").
		Where("organization_id = ?", organizationID).
		Where("kind = ?", domain.RoleKindAdmin).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return "", err
	}
	return role.ID, nil
}

// ensureActiveAdministratorRemains 校验企业仍有正常状态的管理员。
func ensureActiveAdministratorRemains(ctx context.Context, db bun.IDB, organizationID, administratorRoleID string) error {
	count, err := db.NewSelect().TableExpr("users AS u").
		Join("JOIN organization_members AS om ON om.id = u.id AND om.organization_id = u.organization_id AND om.type = ?", domain.MemberIdentityTypeUser).
		Where("u.organization_id = ?", organizationID).
		Where("u.role_id = ?", administratorRoleID).
		Where("om.status = ?", domain.UserStatusActive).
		Count(ctx)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrLastActiveAdministrator
	}
	return nil
}

// loadUserTeams 读取成员所属的全部团队。
func loadUserTeams(ctx context.Context, db bun.IDB, organizationID, userID string) ([]TeamSummary, error) {
	teams := make([]TeamSummary, 0)
	err := db.NewSelect().TableExpr("team_members AS tm").
		ColumnExpr("t.id::text AS id, t.name").
		Join("JOIN teams AS t ON t.id = tm.team_id AND t.organization_id = tm.organization_id").
		Where("tm.organization_id = ?", organizationID).
		Where("tm.member_id = ?", userID).
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
			Where("member_id = ?", userID).
			Exec(ctx)
		return err
	}
	if _, err := tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
		Where("organization_id = ?", identity.Organization.ID).
		Where("member_id = ?", userID).
		Where("team_id NOT IN (?)", bun.In(ids)).
		Exec(ctx); err != nil {
		return err
	}
	relations := make([]servermodels.TeamMember, 0, len(ids))
	for _, teamID := range ids {
		relations = append(relations, servermodels.TeamMember{OrganizationID: identity.Organization.ID, TeamID: teamID, MemberID: userID, CreatedByUserID: identity.User.ID})
	}
	if _, err := tx.NewInsert().Model(&relations).
		Column("organization_id", "team_id", "member_id", "created_by_user_id").
		On("CONFLICT (organization_id, team_id, member_id) DO NOTHING").
		Exec(ctx); err != nil {
		return fmt.Errorf("insert user teams: %w", err)
	}
	return nil
}
