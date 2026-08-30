package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// ChannelType 表示渠道类型。
type ChannelType string

const (
	ChannelTypeWebsite               ChannelType = ChannelType(domain.ChannelTypeWebsite)
	ChannelTypeTelegram              ChannelType = ChannelType(domain.ChannelTypeTelegram)
	ChannelTypeWeChatOfficialAccount ChannelType = ChannelType(domain.ChannelTypeWeChatOfficialAccount)
)

// ChannelRoutingTargetType 表示渠道会话流转目标类型。
type ChannelRoutingTargetType string

const (
	ChannelRoutingTargetTypePublicQueue ChannelRoutingTargetType = ChannelRoutingTargetType(domain.ChannelRoutingTargetTypePublicQueue)
	ChannelRoutingTargetTypeTeam        ChannelRoutingTargetType = ChannelRoutingTargetType(domain.ChannelRoutingTargetTypeTeam)
	ChannelRoutingTargetTypeMember      ChannelRoutingTargetType = ChannelRoutingTargetType(domain.ChannelRoutingTargetTypeMember)
)

// MessageChannelSummary 定义消息渠道列表项和基础信息。
type MessageChannelSummary struct {
	ID                    string               `json:"id"`
	OrganizationID        string               `json:"organizationId"`
	CreatedByUserID       string               `json:"createdByUserId"`
	Type                  ChannelType          `json:"type"`
	Name                  string               `json:"name"`
	Description           *string              `json:"description"`
	DefaultLocale         Locale               `json:"defaultLocale"`
	NewConversationTarget ChannelRoutingTarget `json:"newConversationTarget"`
	FallbackTarget        ChannelRoutingTarget `json:"fallbackTarget"`
	Enabled               bool                 `json:"enabled"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}

// ChannelRoutingTarget 定义渠道会话流转目标。
type ChannelRoutingTarget struct {
	Type ChannelRoutingTargetType `json:"type"`
	ID   string                   `json:"id"`
}

// WebsiteChannel 定义网站渠道详情。
type WebsiteChannel struct {
	MessageChannelSummary
	ChatInterface WebsiteChannelChatInterface `json:"chatInterface"`
	Access        WebsiteChannelAccess        `json:"access"`
}

// TelegramWebhookStatus 表示 Telegram Webhook 的连接状态。
type TelegramWebhookStatus string

const (
	TelegramWebhookStatusWaiting TelegramWebhookStatus = TelegramWebhookStatus(domain.TelegramWebhookStatusWaiting)
	TelegramWebhookStatusNormal  TelegramWebhookStatus = TelegramWebhookStatus(domain.TelegramWebhookStatusNormal)
)

// TelegramChannel 定义 Telegram 渠道详情。
type TelegramChannel struct {
	MessageChannelSummary
	Connection TelegramChannelConnection `json:"connection"`
}

// TelegramChannelConnection 定义 Telegram 机器人和 Webhook 信息。
type TelegramChannelConnection struct {
	BotToken       string                 `json:"botToken"`
	BotID          *string                `json:"botId"`
	BotUsername    *string                `json:"botUsername"`
	BotDisplayName *string                `json:"botDisplayName"`
	WebhookURL     string                 `json:"webhookUrl"`
	WebhookSecret  string                 `json:"webhookSecret"`
	WebhookStatus  *TelegramWebhookStatus `json:"webhookStatus"`
}

// TelegramChannelConnectionInput 定义 Telegram 连接保存输入。
type TelegramChannelConnectionInput struct {
	BotToken        string `json:"botToken"`
	WebhookBaseURL  string `json:"webhookBaseURL"`
	ConfirmBotReuse bool   `json:"confirmBotReuse"`
}

// TelegramChannelConnectionTestInput 定义 Telegram 草稿连接测试输入。
type TelegramChannelConnectionTestInput struct {
	BotToken string `json:"botToken"`
}

// MessageChannelInput 定义消息渠道可编辑的通用字段。
type MessageChannelInput struct {
	Name                  string               `json:"name"`
	Description           string               `json:"description"`
	DefaultLocale         Locale               `json:"defaultLocale"`
	NewConversationTarget ChannelRoutingTarget `json:"newConversationTarget"`
	FallbackTarget        ChannelRoutingTarget `json:"fallbackTarget"`
}

// CreateMessageChannelInput 定义创建消息渠道所需字段。
type CreateMessageChannelInput struct {
	MessageChannelInput
	Type ChannelType `json:"type"`
}

// WebsiteChannelChatInterface 定义网站渠道访客界面设置。
type WebsiteChannelChatInterface struct {
	Title           string  `json:"title"`
	Subtitle        *string `json:"subtitle"`
	GreetingMessage *string `json:"greetingMessage"`
	ThemeColor      string  `json:"themeColor"`
}

// WebsiteChannelChatInterfaceInput 定义网站渠道访客界面输入。
type WebsiteChannelChatInterfaceInput struct {
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	GreetingMessage string `json:"greetingMessage"`
	ThemeColor      string `json:"themeColor"`
}

// WebsiteChannelAccess 定义网站渠道允许使用的网站。
type WebsiteChannelAccess struct {
	AllowedHosts []string `json:"allowedHosts"`
}

// WebsiteChannelAccessInput 定义网站渠道允许使用的网站输入。
type WebsiteChannelAccessInput struct {
	AllowedHosts []string `json:"allowedHosts"`
}

// ChannelOption 定义渠道选择项。
type ChannelOption struct {
	ID   string      `json:"id"`
	Type ChannelType `json:"type"`
	Name string      `json:"name"`
}

// ChannelOptionList 定义渠道选择项列表。
type ChannelOptionList struct {
	Channels []ChannelOption `json:"channels"`
}

// MessageChannelList 定义消息渠道列表。
type MessageChannelList struct {
	Channels []MessageChannelSummary `json:"channels"`
}
