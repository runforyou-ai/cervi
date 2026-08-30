//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

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
func NewWebsiteVisitorDirectBackend(db *bun.DB) *WebsiteVisitorDirectBackend {
	return &WebsiteVisitorDirectBackend{
		listConversations: conversationaction.NewListWebsiteConversationsQuery(db),
		sendTextMessage:   conversationaction.NewReceiveWebsiteCustomerTextMessageAction(db),
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
		ClientMessageID: input.ClientMessageID, Body: input.Body,
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
		point, valid := decodeWebsiteMessageCursor(input.Before, conversationID)
		if !valid {
			return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"before": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorWebsiteMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
		}
		actionInput.Before = &point
	}
	if input.After != "" {
		point, valid := decodeWebsiteMessageCursor(input.After, conversationID)
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
		value := encodeWebsiteMessageCursor(conversationID, *page.Before)
		result.Before = &value
	}
	if page.After != nil {
		value := encodeWebsiteMessageCursor(conversationID, *page.After)
		result.After = &value
	}
	return result, nil
}

// encodeWebsiteMessageCursor 编码绑定 Conversation 的消息位置。
func encodeWebsiteMessageCursor(conversationID string, point conversationaction.MessageCursorPoint) string {
	return conversationID + "." + strconv.FormatInt(point.OriginatedAt.UnixNano(), 10) + "." + strconv.FormatInt(point.SourceOrder, 10) + "." + point.ID
}

// decodeWebsiteMessageCursor 解析当前 Conversation 的消息位置。
func decodeWebsiteMessageCursor(value, conversationID string) (conversationaction.MessageCursorPoint, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != conversationID {
		return conversationaction.MessageCursorPoint{}, false
	}
	originatedAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || originatedAt <= 0 {
		return conversationaction.MessageCursorPoint{}, false
	}
	sourceOrder, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || sourceOrder < 0 {
		return conversationaction.MessageCursorPoint{}, false
	}
	return conversationaction.MessageCursorPoint{OriginatedAt: time.Unix(0, originatedAt).UTC(), SourceOrder: sourceOrder, ID: parts[3]}, true
}

// websiteVisitorError 把语言无关访客错误映射为本地化应用错误。
func websiteVisitorError(ctx context.Context, meta WebsiteVisitorMeta, err error, failureKey cervii18n.Key, operation string, attributes ...any) error {
	requestMeta := RequestMeta{Locale: meta.Locale}
	var validation *conversationaction.ValidationError
	if errors.As(err, &validation) {
		return InvalidError(requestMeta, cervii18n.ErrorValidationFailed, translateValidationFields(validation.Fields, websiteVisitorValidationKeys))
	}
	if errors.Is(err, conversationaction.ErrChannelNotFound) {
		return NotFoundError(requestMeta, cervii18n.ErrorChannelNotFound)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(requestMeta, cervii18n.ErrorWebsiteConversationNotFound)
	}
	var conflict *conversationaction.ConflictError
	if errors.As(err, &conflict) {
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
	conversationaction.ValidationChannelIDInvalid:       cervii18n.FieldChannelIDInvalid,
	conversationaction.ValidationExternalIDInvalid:      cervii18n.FieldVisitorTokenInvalid,
	conversationaction.ValidationConversationIDInvalid:  cervii18n.FieldConversationIDInvalid,
	conversationaction.ValidationClientMessageIDInvalid: cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationBodyRequired:           cervii18n.FieldMessageBodyRequired,
	conversationaction.ValidationBodyTooLong:            cervii18n.FieldMessageBodyTooLong,
	conversationaction.ValidationCursorInvalid:          cervii18n.FieldMessageCursorInvalid,
}

// websiteVisitorConversationFromAction 转换访客会话摘要。
func websiteVisitorConversationFromAction(value conversationaction.ConversationSummary) WebsiteVisitorConversation {
	return WebsiteVisitorConversation{
		ID: value.ID, Title: value.Title, Preview: value.Preview, LastMessageAt: value.LastMessageAt,
		ServiceSession: WebsiteVisitorServiceSession{ID: value.ServiceSessionID, Status: string(value.ServiceSessionStatus)},
	}
}

// websiteVisitorMessageFromAction 转换访客消息。
func websiteVisitorMessageFromAction(value conversationaction.Message) WebsiteVisitorMessage {
	return WebsiteVisitorMessage{
		ID: value.ID, Author: string(value.Author), Body: value.Body,
		OriginatedAt: value.OriginatedAt, CreatedAt: value.CreatedAt,
	}
}
