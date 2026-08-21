//go:build server

package channel

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListWebsiteChannelsQuery 读取当前企业的网站渠道。
type ListWebsiteChannelsQuery struct {
	db *bun.DB
}

// NewListWebsiteChannelsQuery 创建网站渠道列表查询。
func NewListWebsiteChannelsQuery(db *bun.DB) *ListWebsiteChannelsQuery {
	return &ListWebsiteChannelsQuery{db: db}
}

// Execute 返回当前企业的全部网站渠道。
func (q *ListWebsiteChannelsQuery) Execute(ctx context.Context, identity *servermodels.Identity) ([]servermodels.Channel, error) {
	channels := make([]servermodels.Channel, 0)
	if err := q.db.NewSelect().
		Model(&channels).
		Where("c.organization_id = ?", identity.Organization.ID).
		Where("c.type = ?", domain.ChannelTypeWebsite).
		OrderExpr("c.updated_at DESC, c.id DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list website channels: %w", err)
	}
	return channels, nil
}
