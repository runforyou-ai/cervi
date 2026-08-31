//go:build server

package user

import (
	"context"
	"database/sql"
	"errors"

	roleaction "github.com/runforyou-ai/cervi/internal/actions/role"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// loadCurrentIdentity 读取当前用户账号及其企业身份。
func loadCurrentIdentity(ctx context.Context, db bun.IDB, organization servermodels.Organization, userID string) (*servermodels.Identity, error) {
	identity := &servermodels.Identity{Organization: organization}
	err := db.NewSelect().TableExpr("users AS u").
		ColumnExpr("u.id::text, u.identity_id::text, u.organization_id::text, u.email, u.status, u.locale, u.time_zone, u.message_notifications_enabled, u.workspace_tabs_enabled").
		ColumnExpr("oi.id::text, oi.organization_id::text, oi.type, oi.role_id::text, oi.display_name, oi.avatar_file_id::text, oi.work_status").
		Join("JOIN organization_identities AS oi ON oi.id = u.identity_id AND oi.organization_id = u.organization_id AND oi.type = ?", domain.OrganizationIdentityTypeUser).
		Where("u.organization_id = ?", organization.ID).
		Where("u.id = ?", userID).
		Scan(ctx,
			&identity.User.ID,
			&identity.User.IdentityID,
			&identity.User.OrganizationID,
			&identity.User.Email,
			&identity.User.Status,
			&identity.User.Locale,
			&identity.User.TimeZone,
			&identity.User.MessageNotificationsEnabled,
			&identity.User.WorkspaceTabsEnabled,
			&identity.OrganizationIdentity.ID,
			&identity.OrganizationIdentity.OrganizationID,
			&identity.OrganizationIdentity.Type,
			&identity.OrganizationIdentity.RoleID,
			&identity.OrganizationIdentity.DisplayName,
			&identity.OrganizationIdentity.AvatarFileID,
			&identity.OrganizationIdentity.WorkStatus,
		)
	return identity, err
}

// loadUser 读取企业成员、角色和所属团队。
func loadUser(ctx context.Context, db bun.IDB, organizationID, userID string) (*User, error) {
	if !common.ValidUUID(userID) {
		return nil, ErrNotFound
	}
	user := &User{}
	err := db.NewSelect().TableExpr("users AS u").
		ColumnExpr("u.id::text AS id, u.identity_id::text AS identity_id").
		ColumnExpr("u.email, u.status, oi.display_name, oi.work_status, oi.created_at").
		ColumnExpr("r.id::text AS role_id, r.kind AS role_kind, r.name AS role_name").
		Join("JOIN organization_identities AS oi ON oi.id = u.identity_id AND oi.organization_id = u.organization_id AND oi.type = ?", domain.OrganizationIdentityTypeUser).
		Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id").
		Where("u.id = ?", userID).
		Where("u.organization_id = ?", organizationID).
		Scan(ctx, user)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.Teams, err = teamaction.LoadIdentityTeams(ctx, db, organizationID, user.IdentityID)
	return user, err
}

// validateRoleID 校验并锁定当前企业的角色。
func validateRoleID(ctx context.Context, db bun.IDB, organizationID, roleID string) error {
	_, err := roleaction.ValidateAssignment(ctx, db, organizationID, roleID, domain.OrganizationIdentityTypeUser)
	if errors.Is(err, roleaction.ErrAssignmentInvalid) || errors.Is(err, roleaction.ErrAgentAdministrator) {
		return &ValidationError{Fields: map[string]ValidationCode{"roleId": ValidationRoleInvalid}}
	}
	return err
}

// lockAdministratorRole 锁定管理员角色以串行维护有效管理员数量。
func lockAdministratorRole(ctx context.Context, db bun.IDB, organizationID string) (string, error) {
	return roleaction.LockAdministratorRole(ctx, db, organizationID)
}

// ensureActiveAdministratorRemains 校验企业仍有正常状态的管理员。
func ensureActiveAdministratorRemains(ctx context.Context, db bun.IDB, organizationID, administratorRoleID string) error {
	return roleaction.EnsureActiveAdministratorRemains(ctx, db, organizationID, administratorRoleID)
}

// validateTeamIDs 规范化团队编号、去重并校验全部团队属于当前企业。
func validateTeamIDs(ctx context.Context, db bun.IDB, organizationID string, teamIDs []string) ([]string, error) {
	ids, valid := common.NormalizeUUIDs(teamIDs)
	if !valid {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"teamIds": ValidationTeamInvalid}}
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

// replaceUserTeams 校验团队编号后按差集修改成员所属团队。
func replaceUserTeams(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, organizationIdentityID string, teamIDs []string) error {
	ids, err := validateTeamIDs(ctx, tx, identity.Organization.ID, teamIDs)
	if err != nil {
		return err
	}
	return teamaction.ReplaceIdentityTeams(ctx, tx, identity, organizationIdentityID, ids)
}
