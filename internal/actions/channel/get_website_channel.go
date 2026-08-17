//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// GetWebsiteChannelQuery 读取当前企业的单个网站渠道。
type GetWebsiteChannelQuery struct {
	db *bun.DB
}

// WebsiteChannelDetail 定义网站渠道详情和访客聊天界面设置。
type WebsiteChannelDetail struct {
	*servermodels.Channel
	ChatInterface servermodels.WebsiteChannelSetting `json:"chatInterface"`
}

// NewGetWebsiteChannelQuery 创建网站渠道详情查询。
func NewGetWebsiteChannelQuery(db *bun.DB) *GetWebsiteChannelQuery {
	return &GetWebsiteChannelQuery{db: db}
}

// Execute 返回当前企业的网站渠道详情。
func (q *GetWebsiteChannelQuery) Execute(ctx context.Context, principal *servermodels.Principal, channelID string) (*WebsiteChannelDetail, error) {
	if !validUUID(channelID) {
		return nil, ErrNotFound
	}
	channel := &servermodels.Channel{}
	err := q.db.NewSelect().
		Model(channel).
		Where("c.id = ?", channelID).
		Where("c.organization_id = ?", principal.Organization.ID).
		Where("c.type = ?", TypeWebsite).
		Where("c.deleted_at IS NULL").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get website channel: %w", err)
	}
	setting := servermodels.WebsiteChannelSetting{}
	if err := q.db.NewSelect().
		Model(&setting).
		Where("wcs.channel_id = ?", channelID).
		Where("wcs.organization_id = ?", principal.Organization.ID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("get website channel settings: %w", err)
	}
	return &WebsiteChannelDetail{Channel: channel, ChatInterface: setting}, nil
}
