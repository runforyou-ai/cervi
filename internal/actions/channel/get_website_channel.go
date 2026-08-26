//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// GetWebsiteChannelQuery 读取当前企业的单个网站渠道。
type GetWebsiteChannelQuery struct {
	db *bun.DB
}

// WebsiteChannelDetail 定义网站渠道详情和访客聊天界面设置。
type WebsiteChannelDetail struct {
	MessageChannelRecord
	ChatInterface WebsiteChannelSettingRecord `json:"chatInterface"`
}

// NewGetWebsiteChannelQuery 创建网站渠道详情查询。
func NewGetWebsiteChannelQuery(db *bun.DB) *GetWebsiteChannelQuery {
	return &GetWebsiteChannelQuery{db: db}
}

// Execute 返回当前企业的网站渠道详情。
func (q *GetWebsiteChannelQuery) Execute(ctx context.Context, identity *servermodels.Identity, channelID string) (*WebsiteChannelDetail, error) {
	if !common.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return nil, err
	}
	channel := &servermodels.Channel{}
	err := q.db.NewSelect().
		Model(channel).
		Where("c.id = ?", channelID).
		Where("c.organization_id = ?", identity.Organization.ID).
		Where("c.type = ?", domain.ChannelTypeWebsite).
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
		Where("wcs.organization_id = ?", identity.Organization.ID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("get website channel settings: %w", err)
	}
	return &WebsiteChannelDetail{
		MessageChannelRecord: *messageChannelRecord(channel),
		ChatInterface:        websiteChannelSettingRecord(&setting),
	}, nil
}
