//go:build server

package server

import (
	"context"
	"database/sql"
	"errors"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

var _ tenant.Resolver = (*TenantResolver)(nil)

// TenantResolver 优先根据访问地址解析企业，并兼容升级前的唯一企业。
type TenantResolver struct {
	db *bun.DB
}

// NewTenantResolver 创建按访问地址查询企业的租户解析器。
func NewTenantResolver(db *bun.DB) *TenantResolver {
	return &TenantResolver{db: db}
}

// Resolve 返回访问地址绑定的企业。
func (r *TenantResolver) Resolve(ctx context.Context, accessHost string) (tenant.Scope, error) {
	accessHost = tenant.NormalizeAccessHost(accessHost)
	if accessHost == "" {
		return tenant.Scope{}, tenant.ErrNotFound
	}
	organization := &servermodels.Organization{}
	err := r.selectByAccessHost(ctx, organization, accessHost)
	if errors.Is(err, sql.ErrNoRows) {
		err = r.selectByAccessHost(ctx, organization, "")
	}
	if errors.Is(err, sql.ErrNoRows) {
		return tenant.Scope{}, tenant.ErrNotFound
	}
	if err != nil {
		return tenant.Scope{}, err
	}
	return tenant.Scope{
		OrganizationID:   organization.ID,
		OrganizationName: organization.Name,
	}, nil
}

// selectByAccessHost 按规范化访问地址读取企业。
func (r *TenantResolver) selectByAccessHost(ctx context.Context, organization *servermodels.Organization, accessHost string) error {
	return r.db.NewSelect().
		Model(organization).
		Column("id", "name").
		Where("o.access_host = ?", accessHost).
		Limit(1).
		Scan(ctx)
}
