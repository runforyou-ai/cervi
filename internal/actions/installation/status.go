//go:build server

package installation

import (
	"context"
	"errors"

	"github.com/runforyou-ai/cervi/internal/tenant"
)

// Status 表示当前实例是否已初始化以及公开的企业名称。
type Status struct {
	Installed        bool
	OrganizationName string
}

// StatusQuery 查询当前实例的安装状态。
type StatusQuery struct {
	resolveTenant tenant.Resolver
}

// NewStatusQuery 创建安装状态查询。
func NewStatusQuery(tenantResolver tenant.Resolver) *StatusQuery {
	return &StatusQuery{resolveTenant: tenantResolver}
}

// Execute 返回当前实例的初始化状态和公开企业名称。
func (q *StatusQuery) Execute(ctx context.Context) (Status, error) {
	scope, err := q.resolveTenant.Resolve(ctx, tenant.Hostname(ctx))
	if errors.Is(err, tenant.ErrNotFound) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	return Status{Installed: true, OrganizationName: scope.OrganizationName}, nil
}
