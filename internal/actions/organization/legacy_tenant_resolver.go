//go:build server

package organization

import (
	"context"
	"database/sql"
	"errors"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

var _ tenant.Resolver = (*LegacyTenantResolver)(nil)

// LegacyTenantResolver 在域名映射落库前兼容现有单企业数据。
//
// 它只属于迁移期入口，不是租户模型约束。数据库出现多个企业时拒绝猜测；后续
// 域名解析器会替换此实现，认证与业务层继续消费相同的 tenant.Scope。
type LegacyTenantResolver struct {
	db *bun.DB
}

// NewLegacyTenantResolver 创建兼容现有数据的租户解析器。
func NewLegacyTenantResolver(db *bun.DB) *LegacyTenantResolver {
	return &LegacyTenantResolver{db: db}
}

// Resolve 返回现有数据中的唯一企业；hostname 由后续域名解析实现使用。
func (r *LegacyTenantResolver) Resolve(ctx context.Context, hostname string) (tenant.Scope, error) {
	_ = hostname
	organizations := make([]servermodels.Organization, 0, 2)
	err := r.db.NewSelect().
		Model(&organizations).
		Column("id", "name").
		OrderExpr("created_at ASC, id ASC").
		Limit(2).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && len(organizations) == 0) {
		return tenant.Scope{}, tenant.ErrNotFound
	}
	if err != nil {
		return tenant.Scope{}, err
	}
	if len(organizations) > 1 {
		return tenant.Scope{}, tenant.ErrAmbiguous
	}
	return tenant.Scope{
		OrganizationID:   organizations[0].ID,
		OrganizationName: organizations[0].Name,
	}, nil
}
