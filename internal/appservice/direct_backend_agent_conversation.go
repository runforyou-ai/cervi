//go:build server

package appservice

import (
	"context"
	"log/slog"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
)

// SendFirstAgentTextMessage 保存 AI 聊天首条消息并确认草稿对应的会话。
func (b *DirectBackend) SendFirstAgentTextMessage(ctx context.Context, meta RequestMeta, input FirstAgentTextMessageInput) (FirstAgentTextMessageResult, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return FirstAgentTextMessageResult{}, err
	}
	result, err := b.sendFirstAgentTextMessage.Execute(ctx, identity, conversationaction.FirstAgentTextMessageInput{ConversationID: input.ConversationID, AgentIdentityID: input.AgentIdentityID, ClientMessageID: input.ClientMessageID, Body: input.Body})
	if err != nil {
		return FirstAgentTextMessageResult{}, individualConversationError(ctx, meta, err, identity.Organization.ID, input.ConversationID, "start_agent")
	}
	slog.Info("AI 聊天首条消息已保存", "organization_id", identity.Organization.ID, "conversation_id", result.Conversation.ID, "message_id", result.Message.ID, "agent_identity_id", input.AgentIdentityID)
	avatarURLs, err := b.conversationAvatarURLs(ctx, identity, []conversationaction.ConversationMessage{result.Message}, result.Conversation.Agent.AgentAvatarFileID)
	if err != nil {
		slog.Warn("读取 AI 聊天头像失败", "conversation_id", input.ConversationID, "error", err)
	}
	summary := result.Conversation
	return FirstAgentTextMessageResult{
		Conversation: inboxConversationFromAction(summary, avatarURLs),
		Message:      conversationMessageFromAction(result.Message, avatarURLs),
	}, nil
}

// SendAgentTextMessage 保存当前成员在指定 AI 会话中的消息。
func (b *DirectBackend) SendAgentTextMessage(ctx context.Context, meta RequestMeta, conversationID string, input AgentTextMessageInput) (ConversationMessage, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessage{}, err
	}
	message, err := b.sendAgentTextMessage.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID})
	if err != nil {
		return ConversationMessage{}, individualConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "send_agent")
	}
	slog.Info("AI 聊天成员消息已保存", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "message_id", message.ID)
	return b.conversationMessageWithAvatar(ctx, identity, message), nil
}
