//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/uptrace/bun"
)

var _ WebsiteVisitorBackend = (*WebsiteVisitorDirectBackend)(nil)

// WebsiteVisitorDirectBackend 在服务端进程内调用匿名访客 Action 和 Query。
type WebsiteVisitorDirectBackend struct {
	listConversations *conversationaction.ListWebsiteConversationsQuery
	sendTextMessage   *conversationaction.ReceiveWebsiteCustomerTextMessageAction
	listMessages      *conversationaction.ListWebsiteMessagesQuery
}

// NewWebsiteVisitorDirectBackend 创建匿名网站访客直接后端。
func NewWebsiteVisitorDirectBackend(db *bun.DB, agentScheduler conversationaction.CustomerAgentMessageScheduler) *WebsiteVisitorDirectBackend {
	return &WebsiteVisitorDirectBackend{
		listConversations: conversationaction.NewListWebsiteConversationsQuery(db),
		sendTextMessage:   conversationaction.NewReceiveWebsiteCustomerTextMessageAction(db, agentScheduler),
		listMessages:      conversationaction.NewListWebsiteMessagesQuery(db),
	}
}

// ListConversations 返回网站访客的客户会话列表。
func (b *WebsiteVisitorDirectBackend) ListConversations(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string) ([]WebsiteVisitorConversation, error) {
	items, err := b.listConversations.Execute(ctx, channelID, externalID)
	if err != nil {
		return nil, websiteVisitorError(ctx, meta, err, cervii18n.ErrorWebsiteMessengerLoadFailed, "list_conversations", "channel_id", channelID)
	}
	result := make([]WebsiteVisitorConversation, 0, len(items))
	for _, item := range items {
		result = append(result, websiteVisitorConversationFromAction(item))
	}
	return result, nil
}

// SendTextMessage 持久化网站访客文本消息。
func (b *WebsiteVisitorDirectBackend) SendTextMessage(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorTextMessageInput) (WebsiteVisitorTextMessageResult, error) {
	result, err := b.sendTextMessage.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: channelID, ExternalID: externalID, ConversationID: input.ConversationID,
		ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
	})
	if err != nil {
		return WebsiteVisitorTextMessageResult{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorWebsiteMessageSendFailed, "send_text_message", "channel_id", channelID)
	}
	slog.Info("网站访客文本消息已保存",
		"channel_id", channelID,
		"conversation_id", result.Conversation.ID,
		"service_session_id", result.Conversation.ServiceSessionID,
		"message_id", result.Message.ID,
		"created_conversation", result.CreatedConversation,
		"opened_new_service_session", result.OpenedNewServiceSession,
	)
	return WebsiteVisitorTextMessageResult{
		Conversation:            websiteVisitorConversationFromAction(result.Conversation),
		CreatedConversation:     result.CreatedConversation,
		OpenedNewServiceSession: result.OpenedNewServiceSession,
		Message:                 websiteVisitorMessageFromAction(result.Message),
	}, nil
}

// ListMessages 返回网站访客指定客户线程的消息历史。
func (b *WebsiteVisitorDirectBackend) ListMessages(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID string, input WebsiteVisitorMessageHistoryInput) (WebsiteVisitorMessageHistory, error) {
	actionInput := conversationaction.MessageHistoryInput{
		ChannelID: channelID, ExternalID: externalID, ConversationID: conversationID,
	}
	if input.Before != "" && input.After != "" {
		return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"cursor": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorWebsiteMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
	}
	if input.Before != "" {
		point, valid := decodeConversationMessageCursor(input.Before, conversationID)
		if !valid {
			return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"before": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorWebsiteMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
		}
		actionInput.Before = &point
	}
	if input.After != "" {
		point, valid := decodeConversationMessageCursor(input.After, conversationID)
		if !valid {
			return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"after": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorWebsiteMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
		}
		actionInput.After = &point
	}
	page, err := b.listMessages.Execute(ctx, actionInput)
	if err != nil {
		return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorWebsiteMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
	}
	result := WebsiteVisitorMessageHistory{Messages: make([]WebsiteVisitorMessage, 0, len(page.Messages))}
	for _, message := range page.Messages {
		result.Messages = append(result.Messages, websiteVisitorMessageFromAction(message))
	}
	if page.Before != nil {
		value := encodeConversationMessageCursor(conversationID, *page.Before)
		result.Before = &value
	}
	if page.After != nil {
		value := encodeConversationMessageCursor(conversationID, *page.After)
		result.After = &value
	}
	return result, nil
}

// websiteVisitorError 把语言无关访客错误映射为本地化应用错误。
func websiteVisitorError(ctx context.Context, meta WebsiteVisitorMeta, err error, failureKey cervii18n.Key, operation string, attributes ...any) error {
	requestMeta := RequestMeta{Locale: meta.Locale}
	if validation, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(requestMeta, cervii18n.ErrorValidationFailed, translateValidationFields(validation.Fields, websiteVisitorValidationKeys))
	}
	if errors.Is(err, conversationaction.ErrChannelNotFound) {
		return NotFoundError(requestMeta, cervii18n.ErrorChannelNotFound)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(requestMeta, cervii18n.ErrorWebsiteConversationNotFound)
	}
	if conflict, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		if conflict.Reason == conversationaction.ConflictReasonReplyTargetInvalid {
			return ConflictError(requestMeta, cervii18n.ErrorReplyTargetInvalid, conflict.Reason)
		}
		return ConflictError(requestMeta, cervii18n.ErrorWebsiteMessageConflict, conflict.Reason)
	}
	// 请求被取消不属于业务失败，不产生告警日志。
	if ctx.Err() == nil {
		logAttributes := []any{"operation", operation}
		logAttributes = append(logAttributes, attributes...)
		logAttributes = append(logAttributes, "error", err)
		slog.Warn("网站访客操作失败", logAttributes...)
	}
	return FailedError(requestMeta, failureKey)
}

var websiteVisitorValidationKeys = map[conversationaction.ValidationCode]cervii18n.Key{
	conversationaction.ValidationReplyToMessageIDInvalid: cervii18n.FieldReplyToMessageIDInvalid,
	conversationaction.ValidationChannelIDInvalid:        cervii18n.FieldChannelIDInvalid,
	conversationaction.ValidationExternalIDInvalid:       cervii18n.FieldVisitorTokenInvalid,
	conversationaction.ValidationConversationIDInvalid:   cervii18n.FieldConversationIDInvalid,
	conversationaction.ValidationClientMessageIDInvalid:  cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationBodyRequired:            cervii18n.FieldMessageBodyRequired,
	conversationaction.ValidationBodyTooLong:             cervii18n.FieldMessageBodyTooLong,
	conversationaction.ValidationCursorInvalid:           cervii18n.FieldMessageCursorInvalid,
}

// websiteVisitorConversationFromAction 转换访客会话摘要。
func websiteVisitorConversationFromAction(value conversationaction.ConversationSummary) WebsiteVisitorConversation {
	return WebsiteVisitorConversation{
		ID: value.ID, Title: value.Title, Preview: value.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(value.PreviewSenderIdentityType), LastMessageSeq: strconv.FormatInt(value.LastMessageSeq, 10), LastMessageAt: value.LastMessageAt,
		ServiceSession: WebsiteVisitorServiceSession{ID: value.ServiceSessionID, Status: string(value.ServiceSessionStatus)},
	}
}

// websiteVisitorMessageFromAction 转换访客消息。
func websiteVisitorMessageFromAction(value conversationaction.Message) WebsiteVisitorMessage {
	var replyTo *WebsiteVisitorMessageReference
	if value.ReplyTo != nil {
		replyTo = &WebsiteVisitorMessageReference{
			ID: value.ReplyTo.ID, Deleted: value.ReplyTo.Deleted,
			Author: string(value.ReplyTo.Author), Body: value.ReplyTo.Body,
			SenderIdentityType: (*OrganizationIdentityType)(value.ReplyTo.SenderIdentityType),
		}
	}
	return WebsiteVisitorMessage{
		ClientMessageID: value.ClientMessageID,
		ReplyTo:         replyTo,
		ID:              value.ID, Author: string(value.Author), Body: value.Body, SenderIdentityType: (*OrganizationIdentityType)(value.SenderIdentityType),
		MessageSeq: strconv.FormatInt(value.MessageSeq, 10), OriginatedAt: value.OriginatedAt, CreatedAt: value.CreatedAt,
	}
}
