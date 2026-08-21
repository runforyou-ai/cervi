//go:build server

package role

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// validateIdentity 校验当前用户仍属于当前企业。
func validateIdentity(ctx context.Context, db bun.IDB, identity *servermodels.Identity) error {
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return common.ErrIdentityInvalid
	}
	active, err := db.NewSelect().
		Model((*servermodels.User)(nil)).
		Where("id = ?", identity.User.ID).
		Where("organization_id = ?", identity.Organization.ID).
		Where("status = ?", "active").
		Exists(ctx)
	if err != nil {
		return err
	}
	if !active {
		return common.ErrIdentityInvalid
	}
	return nil
}

// loadRole 读取当前企业中的角色。
func loadRole(ctx context.Context, db bun.IDB, organizationID, roleID string, lock bool) (*servermodels.Role, error) {
	if !common.ValidUUID(roleID) {
		return nil, ErrNotFound
	}
	role := &servermodels.Role{}
	query := db.NewSelect().
		Model(role).
		Where("r.id = ?", roleID).
		Where("r.organization_id = ?", organizationID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return role, nil
}

// loadPermissions 读取一组角色的权限。
func loadPermissions(ctx context.Context, db bun.IDB, organizationID string, roleIDs []string) (map[string][]domain.PermissionCode, error) {
	output := make(map[string][]domain.PermissionCode, len(roleIDs))
	if len(roleIDs) == 0 {
		return output, nil
	}
	records := make([]servermodels.RolePermission, 0)
	if err := db.NewSelect().
		Model(&records).
		Where("rp.organization_id = ?", organizationID).
		Where("rp.role_id IN (?)", bun.In(roleIDs)).
		Order("rp.created_at ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	for _, record := range records {
		output[record.RoleID] = append(output[record.RoleID], domain.PermissionCode(record.Permission))
	}
	return output, nil
}

// loadMemberCounts 按成员角色类型统计角色人数。
func loadMemberCounts(ctx context.Context, db bun.IDB, organizationID string, roleIDs []string) (map[string]int, error) {
	output := make(map[string]int, len(roleIDs))
	if len(roleIDs) == 0 {
		return output, nil
	}
	rows := make([]struct {
		RoleID string `bun:"role_id"`
		Count  int    `bun:"member_count"`
	}, 0, len(roleIDs))
	if err := db.NewSelect().
		TableExpr("roles AS r").
		ColumnExpr("r.id::text AS role_id").
		ColumnExpr("count(u.id) AS member_count").
		Join("LEFT JOIN users AS u ON u.organization_id = r.organization_id AND u.role = r.kind").
		Where("r.organization_id = ?", organizationID).
		Where("r.id IN (?)", bun.In(roleIDs)).
		GroupExpr("r.id").
		Scan(ctx, &rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		output[row.RoleID] = row.Count
	}
	return output, nil
}

// replacePermissions 替换角色的全部权限。
func replacePermissions(ctx context.Context, tx bun.Tx, organizationID, roleID string, permissions []domain.PermissionCode) error {
	if _, err := tx.NewDelete().
		Model((*servermodels.RolePermission)(nil)).
		Where("organization_id = ?", organizationID).
		Where("role_id = ?", roleID).
		Exec(ctx); err != nil {
		return err
	}
	if len(permissions) == 0 {
		return nil
	}
	records := make([]servermodels.RolePermission, 0, len(permissions))
	for _, permission := range permissions {
		records = append(records, servermodels.RolePermission{
			OrganizationID: organizationID,
			RoleID:         roleID,
			Permission:     string(permission),
		})
	}
	_, err := tx.NewInsert().
		Model(&records).
		Column("organization_id", "role_id", "permission").
		Exec(ctx)
	return err
}

// recordFromModel 转换角色存储模型。
func recordFromModel(role servermodels.Role, permissions []domain.PermissionCode, memberCount int) Record {
	return Record{
		ID: role.ID, OrganizationID: role.OrganizationID, Kind: domain.RoleKind(role.Kind), Name: role.Name,
		Description: role.Description, Permissions: permissions, MemberCount: memberCount, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt,
	}
}

// isRoleNameConflict 判断自定义角色名称是否重复。
func isRoleNameConflict(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) &&
		postgresError.Field('C') == "23505" &&
		postgresError.Field('n') == "roles_organization_custom_name_unique"
}
