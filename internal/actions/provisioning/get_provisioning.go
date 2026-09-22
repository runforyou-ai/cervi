//go:build server

package provisioning

import (
	"context"
	"strings"

	"github.com/uptrace/bun"
)

// GetProvisioningQuery 按开通标识查询开通结果。
type GetProvisioningQuery struct {
	db *bun.DB
}

// NewGetProvisioningQuery 创建开通结果查询。
func NewGetProvisioningQuery(db *bun.DB) *GetProvisioningQuery {
	return &GetProvisioningQuery{db: db}
}

// Execute 返回开通标识对应企业的当前状态，记录不存在时返回 ErrNotFound。
func (q *GetProvisioningQuery) Execute(ctx context.Context, provisioningID string) (Result, error) {
	result, _, err := loadResult(ctx, q.db, strings.TrimSpace(provisioningID))
	return result, err
}
