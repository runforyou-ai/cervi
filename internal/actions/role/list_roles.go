//go:build server

package role

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListRolesQuery 查询当前企业的角色。
type ListRolesQuery struct {
	db *bun.DB
}

// NewListRolesQuery 创建角色列表查询。
func NewListRolesQuery(db *bun.DB) *ListRolesQuery {
	return &ListRolesQuery{db: db}
}

// Execute 返回角色和预定义权限目录。
func (q *ListRolesQuery) Execute(ctx context.Context, identity *servermodels.Identity) (ListOutput, error) {
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return ListOutput{}, err
	}
	roles := make([]servermodels.Role, 0)
	if err := q.db.NewSelect().
		Model(&roles).
		Where("r.organization_id = ?", identity.Organization.ID).
		OrderExpr("CASE r.kind WHEN 'admin' THEN 0 WHEN 'customer_service' THEN 1 WHEN 'member' THEN 2 ELSE 3 END").
		Order("r.created_at ASC").
		Scan(ctx); err != nil {
		return ListOutput{}, fmt.Errorf("list roles: %w", err)
	}
	roleIDs := make([]string, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}
	permissions, err := loadPermissions(ctx, q.db, identity.Organization.ID, roleIDs)
	if err != nil {
		return ListOutput{}, fmt.Errorf("list role permissions: %w", err)
	}
	memberCounts, err := loadMemberCounts(ctx, q.db, identity.Organization.ID, roleIDs)
	if err != nil {
		return ListOutput{}, fmt.Errorf("count role members: %w", err)
	}
	output := ListOutput{Roles: make([]Record, 0, len(roles)), Permissions: domain.PermissionDefinitions()}
	for _, role := range roles {
		output.Roles = append(output.Roles, recordFromModel(role, permissions[role.ID], memberCounts[role.ID]))
	}
	return output, nil
}
