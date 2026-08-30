//go:build server

package channel

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// MessageChannelInput 定义消息渠道可编辑的通用字段。
type MessageChannelInput struct {
	Name                  string
	Description           string
	DefaultLocale         domain.Locale
	NewConversationTarget RoutingTarget
	FallbackTarget        RoutingTarget
}

// CreateMessageChannelInput 定义创建消息渠道所需字段。
type CreateMessageChannelInput struct {
	MessageChannelInput
	Type domain.ChannelType
}

// RoutingTarget 定义渠道会话流转目标。
type RoutingTarget struct {
	Type domain.ChannelRoutingTargetType
	ID   string
}

// WebsiteChannelChatInterfaceInput 定义网站渠道聊天界面可编辑字段。
type WebsiteChannelChatInterfaceInput struct {
	Title           string
	Subtitle        string
	GreetingMessage string
	ThemeColor      string
}

// WebsiteChannelAccessInput 定义网站渠道允许使用的网站输入。
type WebsiteChannelAccessInput struct {
	AllowedHosts []string
}

// TelegramChannelConnectionInput 定义 Telegram 连接可编辑字段。
type TelegramChannelConnectionInput struct {
	BotToken        string
	WebhookBaseURL  string
	ConfirmBotReuse bool
}

// TelegramChannelConnectionTestInput 定义 Telegram 草稿连接测试字段。
type TelegramChannelConnectionTestInput struct {
	BotToken string
}

// MessageChannelRecord 定义消息渠道传输字段。
type MessageChannelRecord struct {
	ID                        string    `json:"id"`
	OrganizationID            string    `json:"organizationId"`
	CreatedByUserID           string    `json:"createdByUserId"`
	Type                      string    `json:"type"`
	Name                      string    `json:"name"`
	Description               *string   `json:"description"`
	DefaultLocale             string    `json:"defaultLocale"`
	InitialRoutingTargetType  string    `json:"initialRoutingTargetType"`
	InitialRoutingTargetID    *string   `json:"initialRoutingTargetId"`
	FallbackRoutingTargetType string    `json:"fallbackRoutingTargetType"`
	FallbackRoutingTargetID   *string   `json:"fallbackRoutingTargetId"`
	Enabled                   bool      `json:"enabled"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// WebsiteChannelSettingRecord 定义网站渠道访客聊天界面传输字段。
type WebsiteChannelSettingRecord struct {
	ChatTitle         string   `json:"title"`
	ChatSubtitle      *string  `json:"subtitle"`
	GreetingMessage   *string  `json:"greetingMessage"`
	ThemeColor        string   `json:"themeColor"`
	AllowedEmbedHosts []string `json:"allowedHosts"`
}

// TelegramChannelSettingRecord 定义 Telegram 机器人和 Webhook 传输字段。
type TelegramChannelSettingRecord struct {
	BotToken           string     `json:"botToken"`
	BotID              *int64     `json:"botId"`
	BotUsername        *string    `json:"botUsername"`
	BotDisplayName     *string    `json:"botDisplayName"`
	WebhookBaseURL     string     `json:"webhookBaseUrl"`
	WebhookURL         string     `json:"webhookUrl"`
	WebhookSecret      string     `json:"webhookSecret"`
	WebhookStatus      *string    `json:"webhookStatus"`
	WebhookConnectedAt *time.Time `json:"webhookConnectedAt"`
}

// messageChannelRecord 把渠道存储模型转换为传输结构。
func messageChannelRecord(channel *servermodels.Channel) *MessageChannelRecord {
	return &MessageChannelRecord{
		ID:                        channel.ID,
		OrganizationID:            channel.OrganizationID,
		CreatedByUserID:           channel.CreatedByUserID,
		Type:                      channel.Type,
		Name:                      channel.Name,
		Description:               channel.Description,
		DefaultLocale:             channel.DefaultLocale,
		InitialRoutingTargetType:  channel.InitialRoutingTargetType,
		InitialRoutingTargetID:    channel.InitialRoutingTargetID,
		FallbackRoutingTargetType: channel.FallbackRoutingTargetType,
		FallbackRoutingTargetID:   channel.FallbackRoutingTargetID,
		Enabled:                   channel.Enabled,
		CreatedAt:                 channel.CreatedAt,
		UpdatedAt:                 channel.UpdatedAt,
	}
}

// websiteChannelSettingRecord 把网站渠道设置存储模型转换为传输结构。
func websiteChannelSettingRecord(setting *servermodels.WebsiteChannelSetting) WebsiteChannelSettingRecord {
	return WebsiteChannelSettingRecord{
		ChatTitle:         setting.ChatTitle,
		ChatSubtitle:      setting.ChatSubtitle,
		GreetingMessage:   setting.GreetingMessage,
		ThemeColor:        setting.ThemeColor,
		AllowedEmbedHosts: setting.AllowedEmbedHosts,
	}
}

// telegramChannelSettingRecord 把 Telegram 设置存储模型转换为传输结构。
func telegramChannelSettingRecord(setting *servermodels.TelegramChannelSetting) TelegramChannelSettingRecord {
	return TelegramChannelSettingRecord{
		BotToken:           optionalStringValue(setting.BotToken),
		BotID:              setting.BotID,
		BotUsername:        setting.BotUsername,
		BotDisplayName:     setting.BotDisplayName,
		WebhookBaseURL:     optionalStringValue(setting.WebhookBaseURL),
		WebhookSecret:      optionalStringValue(setting.WebhookSecret),
		WebhookStatus:      setting.WebhookStatus,
		WebhookConnectedAt: setting.WebhookConnectedAt,
	}
}

// optionalStringValue 返回可空字符串的实际值。
func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
