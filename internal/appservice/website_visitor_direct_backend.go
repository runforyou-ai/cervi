//go:build server

package appservice

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	cursorKey         [32]byte
}

type websiteMessageCursor struct {
	Direction      string `json:"d"`
	ConversationID string `json:"c"`
	OriginatedAt   int64  `json:"t"`
	MessageID      string `json:"i"`
}

// NewWebsiteVisitorDirectBackend 创建匿名网站访客直接后端。
func NewWebsiteVisitorDirectBackend(db *bun.DB) (*WebsiteVisitorDirectBackend, error) {
	backend := &WebsiteVisitorDirectBackend{
		listConversations: conversationaction.NewListWebsiteConversationsQuery(db),
		sendTextMessage:   conversationaction.NewReceiveWebsiteCustomerTextMessageAction(db),
		listMessages:      conversationaction.NewListWebsiteMessagesQuery(db),
	}
	if _, err := rand.Read(backend.cursorKey[:]); err != nil {
		return nil, fmt.Errorf("generate website message cursor key: %w", err)
	}
	return backend, nil
}

// ListConversations 返回网站访客的客户会话列表。
func (b *WebsiteVisitorDirectBackend) ListConversations(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string) ([]WebsiteVisitorConversation, error) {
	items, err := b.listConversations.Execute(ctx, channelID, externalID)
	if err != nil {
		return nil, b.websiteVisitorError(meta, err, cervii18n.ErrorWebsiteMessengerLoadFailed)
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
		return WebsiteVisitorTextMessageResult{}, b.websiteVisitorError(meta, err, cervii18n.ErrorWebsiteMessageSendFailed)
	}
	slog.Info("收到网站访客文本消息",
		"organization_id", result.OrganizationID,
		"channel_id", result.ChannelID,
		"conversation_id", result.Conversation.ID,
		"service_session_id", result.ServiceSessionID,
		"message_id", result.Message.ID,
		"created_contact", result.CreatedContact,
		"inserted_conversation", result.InsertedConversation,
		"created_service_session", result.CreatedServiceSession,
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
		return WebsiteVisitorMessageHistory{}, b.websiteVisitorError(meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"cursor": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorWebsiteMessageListFailed)
	}
	if input.Before != "" {
		point, err := b.decodeCursor(input.Before, "before", conversationID)
		if err != nil {
			return WebsiteVisitorMessageHistory{}, b.websiteVisitorError(meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"before": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorWebsiteMessageListFailed)
		}
		actionInput.Before = &point
	}
	if input.After != "" {
		point, err := b.decodeCursor(input.After, "after", conversationID)
		if err != nil {
			return WebsiteVisitorMessageHistory{}, b.websiteVisitorError(meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"after": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorWebsiteMessageListFailed)
		}
		actionInput.After = &point
	}
	page, err := b.listMessages.Execute(ctx, actionInput)
	if err != nil {
		return WebsiteVisitorMessageHistory{}, b.websiteVisitorError(meta, err, cervii18n.ErrorWebsiteMessageListFailed)
	}
	result := WebsiteVisitorMessageHistory{Messages: make([]WebsiteVisitorMessage, 0, len(page.Messages))}
	for _, message := range page.Messages {
		result.Messages = append(result.Messages, websiteVisitorMessageFromAction(message))
	}
	if page.Before != nil {
		value, err := b.encodeCursor("before", conversationID, *page.Before)
		if err != nil {
			return WebsiteVisitorMessageHistory{}, b.websiteVisitorError(meta, err, cervii18n.ErrorWebsiteMessageListFailed)
		}
		result.Before = &value
	}
	if page.After != nil {
		value, err := b.encodeCursor("after", conversationID, *page.After)
		if err != nil {
			return WebsiteVisitorMessageHistory{}, b.websiteVisitorError(meta, err, cervii18n.ErrorWebsiteMessageListFailed)
		}
		result.After = &value
	}
	return result, nil
}

// encodeCursor 签名网站消息分页边界。
func (b *WebsiteVisitorDirectBackend) encodeCursor(direction, conversationID string, point conversationaction.MessageCursorPoint) (string, error) {
	payload, err := json.Marshal(websiteMessageCursor{
		Direction: direction, ConversationID: conversationID,
		OriginatedAt: point.OriginatedAt.UnixNano(), MessageID: point.ID,
	})
	if err != nil {
		return "", fmt.Errorf("encode website message cursor: %w", err)
	}
	mac := hmac.New(sha256.New, b.cursorKey[:])
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// decodeCursor 校验并解析网站消息分页边界。
func (b *WebsiteVisitorDirectBackend) decodeCursor(value, direction, conversationID string) (conversationaction.MessageCursorPoint, error) {
	payloadValue, signatureValue, found := strings.Cut(value, ".")
	if !found || strings.Contains(signatureValue, ".") {
		return conversationaction.MessageCursorPoint{}, errors.New("invalid website message cursor")
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadValue)
	if err != nil {
		return conversationaction.MessageCursorPoint{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(signatureValue)
	if err != nil {
		return conversationaction.MessageCursorPoint{}, err
	}
	mac := hmac.New(sha256.New, b.cursorKey[:])
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return conversationaction.MessageCursorPoint{}, errors.New("invalid website message cursor signature")
	}
	var cursor websiteMessageCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.Direction != direction || cursor.ConversationID != conversationID || cursor.OriginatedAt == 0 {
		return conversationaction.MessageCursorPoint{}, errors.New("invalid website message cursor payload")
	}
	return conversationaction.MessageCursorPoint{OriginatedAt: time.Unix(0, cursor.OriginatedAt).UTC(), ID: cursor.MessageID}, nil
}

// websiteVisitorError 把语言无关访客错误映射为本地化应用错误。
func (b *WebsiteVisitorDirectBackend) websiteVisitorError(meta WebsiteVisitorMeta, err error, failureKey cervii18n.Key) error {
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
	slog.Warn("网站访客应用服务调用失败", "error", err)
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
