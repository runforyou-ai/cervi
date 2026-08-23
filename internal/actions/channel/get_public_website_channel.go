//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// GetPublicWebsiteChannelQuery 按公开标识读取网站渠道。
type GetPublicWebsiteChannelQuery struct {
	db *bun.DB
}

// PublicWebsiteChannel 定义访客入口所需的网站渠道信息。
type PublicWebsiteChannel struct {
	ID                string
	Title             string
	Subtitle          string
	Greeting          string
	ThemeColor        string
	AllowedEmbedHosts []string
	DefaultLocale     domain.Locale
}

// NewGetPublicWebsiteChannelQuery 创建公开网站渠道查询。
func NewGetPublicWebsiteChannelQuery(db *bun.DB) *GetPublicWebsiteChannelQuery {
	return &GetPublicWebsiteChannelQuery{db: db}
}

// Execute 返回已启用的网站渠道访客界面设置。
func (q *GetPublicWebsiteChannelQuery) Execute(ctx context.Context, channelID string) (*PublicWebsiteChannel, error) {
	if !common.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	channel := &servermodels.Channel{}
	err := q.db.NewSelect().
		Model(channel).
		Column("id", "default_locale").
		Where("c.id = ?", channelID).
		Where("c.type = ?", domain.ChannelTypeWebsite).
		Where("c.enabled = TRUE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get public website channel: %w", err)
	}
	setting := servermodels.WebsiteChannelSetting{}
	if err := q.db.NewSelect().
		Model(&setting).
		Column("chat_title", "chat_subtitle", "greeting_message", "theme_color", "allowed_embed_hosts").
		Where("wcs.channel_id = ?", channel.ID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("get public website channel settings: %w", err)
	}
	return &PublicWebsiteChannel{
		ID:                channel.ID,
		Title:             setting.ChatTitle,
		Subtitle:          derefText(setting.ChatSubtitle),
		Greeting:          derefText(setting.GreetingMessage),
		ThemeColor:        setting.ThemeColor,
		AllowedEmbedHosts: setting.AllowedEmbedHosts,
		DefaultLocale:     domain.Locale(channel.DefaultLocale),
	}, nil
}

// derefText 把可空字符串读成空串或原值。
func derefText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
