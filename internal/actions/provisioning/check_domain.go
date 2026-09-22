//go:build server

package provisioning

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Availability 描述域名前缀在查询时刻的可用状态。
type Availability struct {
	DomainPrefix string
	AccessHost   string
	Available    bool
}

// CheckDomainQuery 查询托管企业域名前缀是否已登记。
type CheckDomainQuery struct {
	db           *bun.DB
	domainSuffix string
}

// NewCheckDomainQuery 创建域名前缀可用性查询。
func NewCheckDomainQuery(db *bun.DB, domainSuffix string) *CheckDomainQuery {
	return &CheckDomainQuery{db: db, domainSuffix: domainSuffix}
}

// Execute 规范化并校验前缀，返回对应企业域名是否尚未登记。
func (q *CheckDomainQuery) Execute(ctx context.Context, prefix string) (Availability, error) {
	prefix = domain.NormalizeDomainPrefix(prefix)
	if !domain.DomainPrefixValid(prefix) {
		return Availability{}, ErrDomainPrefixInvalid
	}
	accessHost := domain.ManagedAccessHost(prefix, q.domainSuffix)
	taken, err := q.db.NewSelect().
		Model((*servermodels.Organization)(nil)).
		Where("o.access_host = ?", accessHost).
		Exists(ctx)
	if err != nil {
		return Availability{}, err
	}
	return Availability{DomainPrefix: prefix, AccessHost: accessHost, Available: !taken}, nil
}
