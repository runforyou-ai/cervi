//go:build server

package channel

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Summary 定义供目录和选择器使用的渠道摘要。
type Summary struct {
	ID   string `bun:"id" json:"id"`
	Type string `bun:"type" json:"type"`
	Name string `bun:"name" json:"name"`
}

// ListChannelsQuery 读取当前企业的所有有效渠道。
type ListChannelsQuery struct {
	db *bun.DB
}

// NewListChannelsQuery 创建通用渠道列表查询。
func NewListChannelsQuery(db *bun.DB) *ListChannelsQuery {
	return &ListChannelsQuery{db: db}
}

// Execute 返回当前企业未删除的渠道摘要。
func (q *ListChannelsQuery) Execute(ctx context.Context, principal *servermodels.Principal) ([]Summary, error) {
	channels := make([]Summary, 0)
	if err := q.db.NewSelect().
		TableExpr("channels AS c").
		ColumnExpr("c.id::text AS id").
		ColumnExpr("c.type").
		ColumnExpr("c.name").
		Where("c.organization_id = ?", principal.Organization.ID).
		Where("c.deleted_at IS NULL").
		OrderExpr("c.type ASC, lower(c.name) ASC, c.id ASC").
		Scan(ctx, &channels); err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return channels, nil
}
