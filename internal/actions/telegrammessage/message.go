//go:build server

// Package telegrammessage 维护 Telegram 平台消息身份与本地引用关系。
package telegrammessage

import (
	"context"
	"fmt"

	models "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Inbound 定义 Telegram 入站消息的稳定平台事实。
type Inbound struct {
	BotID     int64
	ChatID    int64
	MessageID int64
	Reply     *Reply
}

// Reply 保存同一聊天内被引用消息的编号和一层快照。
type Reply struct {
	MessageID   int64
	Body        string
	SenderName  string
	SenderIsBot bool
}

// Record 在渠道身份锁保护的事务中保存映射，并关联先到达的引用消息。
func Record(ctx context.Context, db bun.IDB, record *models.TelegramMessage) error {
	if _, err := db.NewInsert().Model(record).Exec(ctx); err != nil {
		return err
	}
	// 当前消息和其原消息任意一方后到达时，都按完整平台身份关联。
	_, err := db.ExecContext(ctx, `UPDATE messages AS msg SET reply_to_message_id = target.message_id
        FROM telegram_messages AS source
        JOIN telegram_messages AS target ON target.organization_id = source.organization_id
            AND target.conversation_id = source.conversation_id AND target.channel_id = source.channel_id
            AND target.bot_id = source.bot_id AND target.chat_id = source.chat_id
            AND target.provider_message_id = source.reply_provider_message_id
        WHERE msg.id = source.message_id AND msg.organization_id = source.organization_id
            AND msg.conversation_id = source.conversation_id AND msg.reply_to_message_id IS NULL
            AND source.organization_id = ? AND source.channel_id = ? AND source.bot_id = ? AND source.chat_id = ?
            AND (source.message_id = ? OR target.message_id = ?)`,
		record.OrganizationID, record.ChannelID, record.BotID, record.ChatID, record.MessageID, record.MessageID)
	return err
}

// RecordInbound 保存新接收文本消息的平台身份和引用快照。
func RecordInbound(ctx context.Context, db bun.IDB, channelID string, message *models.Message, input *Inbound) error {
	record := &models.TelegramMessage{
		MessageID: message.ID, OrganizationID: message.OrganizationID, ConversationID: message.ConversationID,
		ChannelID: channelID, BotID: input.BotID, ChatID: input.ChatID, ProviderMessageID: input.MessageID,
	}
	if input.Reply != nil {
		record.ReplyProviderMessageID = &input.Reply.MessageID
		record.ReplyBody, record.ReplySenderName, record.ReplySenderIsBot = input.Reply.Body, input.Reply.SenderName, input.Reply.SenderIsBot
	}
	return Record(ctx, db, record)
}

// MatchesInbound 按稳定平台身份核对重放，不依赖后来补齐的本地引用编号或可变原文。
func MatchesInbound(ctx context.Context, db bun.IDB, message *models.Message, channelID string, input *Inbound) (bool, error) {
	var record models.TelegramMessage
	if err := db.NewSelect().Model(&record).Where("tm.message_id = ? AND tm.organization_id = ?", message.ID, message.OrganizationID).Scan(ctx); err != nil {
		return false, fmt.Errorf("load telegram message mapping: %w", err)
	}
	var storedReply, incomingReply int64
	if record.ReplyProviderMessageID != nil {
		storedReply = *record.ReplyProviderMessageID
	}
	if input.Reply != nil {
		incomingReply = input.Reply.MessageID
	}
	return record.ChannelID == channelID && record.BotID == input.BotID && record.ChatID == input.ChatID && record.ProviderMessageID == input.MessageID && storedReply == incomingReply, nil
}
