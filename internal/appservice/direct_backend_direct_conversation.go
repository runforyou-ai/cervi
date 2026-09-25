//go:build server

// 真人单聊操作与双方聊天错误转换。
package appservice

import (
	"context"
	"errors"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"log/slog"
)

// SendFirstDirectTextMessage 发送首条单聊消息并按需创建长期会话。
func (o *directOperations) SendFirstDirectTextMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input FirstDirectTextMessageInput) (FirstDirectTextMessageResult, error) {
	result, err := o.sendFirstDirectTextMessage.Execute(ctx, identity, conversationaction.FirstDirectTextMessageInput{
		TargetIdentityID: input.TargetIdentityID, ClientMessageID: input.ClientMessageID, Body: input.Body,
	})
	if err != nil {
		return FirstDirectTextMessageResult{}, individualConversationError(ctx, meta, err, identity.Organization.ID, input.TargetIdentityID, "send_first")
	}
	slog.Info("企业成员内部单聊首条文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", result.Conversation.ID,
		"target_identity_id", result.Conversation.PeerIdentityID,
		"message_id", result.Message.ID,
	)
	avatarURLs, err := o.conversationAvatarURLs(ctx, identity, []conversationaction.ConversationMessage{result.Message}, result.Conversation.PeerAvatarFileID)
	if err != nil {
		slog.Warn("读取已保存单聊首条消息头像失败", "organization_id", identity.Organization.ID, "conversation_id", result.Conversation.ID, "message_id", result.Message.ID, "error", err)
	}
	return FirstDirectTextMessageResult{
		Conversation: directInboxConversationFromSummary(result.Conversation, avatarURLs),
		Message:      conversationMessageFromAction(result.Message, avatarURLs),
	}, nil
}

// FindDirectConversation 按目标身份查找当前成员的活跃单聊。
func (o *directOperations) FindDirectConversation(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, targetIdentityID string) (DirectConversationLookup, error) {
	summary, err := o.findDirectConversation.Execute(ctx, identity, targetIdentityID)
	if err != nil {
		return DirectConversationLookup{}, individualConversationError(ctx, meta, err, identity.Organization.ID, targetIdentityID, "find")
	}
	if summary == nil {
		return DirectConversationLookup{}, nil
	}
	avatarURLs, err := o.conversationAvatarURLs(ctx, identity, nil, summary.PeerAvatarFileID)
	if err != nil {
		return DirectConversationLookup{}, individualConversationError(ctx, meta, err, identity.Organization.ID, targetIdentityID, "find")
	}
	conversation := directInboxConversationFromSummary(*summary, avatarURLs)
	return DirectConversationLookup{Conversation: &conversation}, nil
}

// directInboxConversationFromSummary 把单聊摘要转换为统一收件箱会话。
func directInboxConversationFromSummary(summary conversationaction.DirectConversationSummary, avatarURLs map[string]string) InboxConversation {
	return InboxConversation{
		ID: summary.ID, Type: ConversationTypeDirect, LastActivityAt: summary.LastActivityAt,
		Direct: &DirectInboxConversation{
			PeerIdentityID: summary.PeerIdentityID, PeerType: OrganizationIdentityType(summary.PeerType), PeerName: summary.PeerName, PeerAvatarURL: optionalFileURL(avatarURLs, summary.PeerAvatarFileID),
			Preview: messagePreviewText(summary.Preview, summary.PreviewSenderIdentityType), LastMessageAt: summary.LastMessageAt,
		},
	}
}

// SendDirectTextMessage 发送内部单聊文本消息。
func (o *directOperations) SendDirectTextMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input DirectTextMessageInput) (ConversationMessage, error) {
	message, err := o.sendDirectTextMessage.Execute(ctx, identity, conversationaction.InternalTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
	})
	if err != nil {
		return ConversationMessage{}, individualConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "send")
	}
	slog.Info("企业成员内部单聊文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"sender_identity_id", identity.OrganizationIdentity.ID,
	)
	return o.conversationMessageWithAvatar(ctx, identity, message), nil
}

// individualConversationError 转换真人单聊和 AI 聊天的目标及消息错误。
func individualConversationError(ctx context.Context, meta RequestMeta, err error, organizationID, targetID, operation string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrAgentTargetNotFound) {
		return NotFoundError(meta, cervii18n.ErrorAgentNotFound)
	}
	if errors.Is(err, conversationaction.ErrAgentUnavailable) {
		return NotFoundError(meta, cervii18n.ErrorAgentUnavailable)
	}
	if errors.Is(err, conversationaction.ErrDirectTargetNotFound) {
		return NotFoundError(meta, cervii18n.ErrorDirectTargetNotFound)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	if conflictError, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		if conflictError.Reason == conversationaction.ConflictReasonReplyTargetInvalid {
			return ConflictError(meta, cervii18n.ErrorReplyTargetInvalid, conflictError.Reason)
		}
		if key, ok := assistantConflictKeys[conflictError.Reason]; ok {
			return ConflictError(meta, key, conflictError.Reason)
		}
		return ConflictError(meta, cervii18n.ErrorMessageConflict, conflictError.Reason)
	}
	slog.Warn("双方聊天操作失败", "organization_id", organizationID, "target_id", targetID, "operation", operation, "error", err)
	if operation == "find" {
		return FailedError(meta, cervii18n.ErrorDirectConversationLookupFailed)
	}
	return FailedError(meta, cervii18n.ErrorDirectMessageSendFailed)
}
