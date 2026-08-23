//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// WebsiteChannelSetting 表示网站渠道的访客聊天界面设置。
type WebsiteChannelSetting struct {
	bun.BaseModel `bun:"table:website_channel_settings,alias:wcs"`

	ChannelID         string    `bun:"channel_id,pk" json:"-"`
	OrganizationID    string    `bun:"organization_id" json:"-"`
	ChatTitle         string    `bun:"chat_title" json:"title"`
	ChatSubtitle      *string   `bun:"chat_subtitle" json:"subtitle"`
	GreetingMessage   *string   `bun:"greeting_message" json:"greetingMessage"`
	ThemeColor        string    `bun:"theme_color" json:"themeColor"`
	AllowedEmbedHosts []string  `bun:"allowed_embed_hosts,array" json:"allowedHosts"`
	CreatedAt         time.Time `bun:"created_at" json:"-"`
	UpdatedAt         time.Time `bun:"updated_at" json:"-"`
}
