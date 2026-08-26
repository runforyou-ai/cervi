//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// WebsiteChannelSetting 表示网站渠道的访客聊天界面设置。
type WebsiteChannelSetting struct {
	bun.BaseModel `bun:"table:website_channel_settings,alias:wcs"`

	ChannelID         string    `bun:"channel_id,pk"`
	OrganizationID    string    `bun:"organization_id"`
	ChatTitle         string    `bun:"chat_title"`
	ChatSubtitle      *string   `bun:"chat_subtitle"`
	GreetingMessage   *string   `bun:"greeting_message"`
	ThemeColor        string    `bun:"theme_color"`
	AllowedEmbedHosts []string  `bun:"allowed_embed_hosts,array"`
	CreatedAt         time.Time `bun:"created_at"`
	UpdatedAt         time.Time `bun:"updated_at"`
}
