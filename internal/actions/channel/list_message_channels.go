//go:build server

package channel

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListMessageChannelsQuery 读取当前企业的消息渠道。
type ListMessageChannelsQuery struct {
	db *bun.DB
}

// NewListMessageChannelsQuery 创建消息渠道列表查询。
func NewListMessageChannelsQuery(db *bun.DB) *ListMessageChannelsQuery {
	return &ListMessageChannelsQuery{db: db}
}

// Execute 返回当前企业支持管理的全部消息渠道。
func (q *ListMessageChannelsQuery) Execute(ctx context.Context, identity *servermodels.Identity) ([]servermodels.Channel, error) {
	channels := make([]servermodels.Channel, 0)
	if err := q.db.NewSelect().
		Model(&channels).
		Where("c.organization_id = ?", identity.Organization.ID).
		Where("c.type IN (?)", bun.In(domain.MessageChannelTypes())).
		OrderExpr("c.updated_at DESC, c.id DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list message channels: %w", err)
	}
	return channels, nil
}
