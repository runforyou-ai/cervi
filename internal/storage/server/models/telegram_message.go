//go:build server

package models

import "github.com/uptrace/bun"

// TelegramMessage 保存平台消息身份与入站引用的一层快照。
type TelegramMessage struct {
	bun.BaseModel          `bun:"table:telegram_messages,alias:tm"`
	MessageID              string `bun:"message_id,pk"`
	OrganizationID         string `bun:"organization_id"`
	ConversationID         string `bun:"conversation_id"`
	ChannelID              string `bun:"channel_id"`
	BotID                  int64  `bun:"bot_id"`
	ChatID                 int64  `bun:"chat_id"`
	ProviderMessageID      int64  `bun:"provider_message_id"`
	ReplyProviderMessageID *int64 `bun:"reply_provider_message_id"`
	ReplyBody              string `bun:"reply_body"`
	ReplySenderName        string `bun:"reply_sender_name"`
	ReplySenderIsBot       bool   `bun:"reply_sender_is_bot"`
}
