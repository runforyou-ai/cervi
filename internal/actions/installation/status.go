//go:build server

package installation

import (
	"context"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// StatusQuery 查询当前实例的安装状态。
type StatusQuery struct {
	db *bun.DB
}

// NewStatusQuery 创建安装状态查询。
func NewStatusQuery(db *bun.DB) *StatusQuery {
	return &StatusQuery{db: db}
}

// Execute 返回当前实例是否已完成初始化。
func (q *StatusQuery) Execute(ctx context.Context) (bool, error) {
	return q.db.NewSelect().Model((*servermodels.Organization)(nil)).Exists(ctx)
}
