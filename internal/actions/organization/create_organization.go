//go:build server

package organization

import (
	"context"
	"errors"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

// ErrAccessHostTaken 表示访问地址已登记给其他企业。
var ErrAccessHostTaken = errors.New("organization access host is taken")

// CreateInput 定义创建企业及首位管理员所需的已校验字段。
type CreateInput struct {
	AccessHost        string
	Name              string
	AdminDisplayName  string
	AdminEmail        string
	AdminPasswordHash string
	Locale            domain.Locale
	TimeZone          string
}

// Create 在调用方事务内创建企业、内置角色与默认权限和首位管理员；AdminPasswordHash 为空时管理员没有本地密码。
func Create(ctx context.Context, tx bun.Tx, input CreateInput) (*servermodels.Identity, error) {
	organization := &servermodels.Organization{AccessHost: input.AccessHost, Name: input.Name}
	if _, err := tx.NewInsert().
		Model(organization).
		Column("access_host", "name").
		Returning("id, lifecycle_status, created_at, updated_at").
		Exec(ctx); err != nil {
		if pgerr.UniqueViolationOn(err, "organizations_access_host_unique") {
			return nil, ErrAccessHostTaken
		}
		return nil, err
	}

	var adminRoleID string
	for _, kind := range domain.BuiltInRoleKinds() {
		role := &servermodels.Role{OrganizationID: organization.ID, Kind: string(kind)}
		if _, err := tx.NewInsert().
			Model(role).
			Column("organization_id", "kind").
			Returning("id").
			Exec(ctx); err != nil {
			return nil, err
		}
		if kind == domain.RoleKindAdmin {
			adminRoleID = role.ID
		}
		permissions := domain.DefaultRolePermissions(kind)
		if len(permissions) == 0 {
			continue
		}
		records := make([]servermodels.RolePermission, 0, len(permissions))
		for _, permission := range permissions {
			records = append(records, servermodels.RolePermission{
				OrganizationID: organization.ID,
				RoleID:         role.ID,
				Permission:     string(permission),
			})
		}
		if _, err := tx.NewInsert().
			Model(&records).
			Column("organization_id", "role_id", "permission").
			Exec(ctx); err != nil {
			return nil, err
		}
	}

	// 企业创建者默认开启接待客户，企业创建后即可处理客户会话。
	organizationIdentity := &servermodels.OrganizationIdentity{
		OrganizationID:   organization.ID,
		Type:             string(domain.OrganizationIdentityTypeUser),
		RoleID:           adminRoleID,
		DisplayName:      input.AdminDisplayName,
		HandlesCustomers: true,
		WorkStatus:       string(domain.WorkStatusWorking),
	}
	if _, err := tx.NewInsert().Model(organizationIdentity).
		Column("organization_id", "type", "role_id", "display_name", "handles_customers", "work_status").
		Returning("id, work_status, work_status_updated_at").Exec(ctx); err != nil {
		return nil, err
	}
	user := &servermodels.User{
		IdentityID:     organizationIdentity.ID,
		OrganizationID: organization.ID,
		Email:          input.AdminEmail,
		PasswordHash:   input.AdminPasswordHash,
		Status:         string(domain.UserStatusActive),
		Locale:         string(input.Locale),
		TimeZone:       input.TimeZone,
	}
	if _, err := tx.NewInsert().
		Model(user).
		Column("identity_id", "organization_id", "email", "password_hash", "status", "locale", "time_zone").
		Returning("id, message_notifications_enabled").
		Exec(ctx); err != nil {
		return nil, err
	}
	return &servermodels.Identity{
		Organization:         *organization,
		OrganizationIdentity: *organizationIdentity,
		User:                 *user,
	}, nil
}
