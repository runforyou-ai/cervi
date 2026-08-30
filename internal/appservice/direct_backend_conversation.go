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
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// SendCustomerTextMessage 发送成员客户会话文本消息。
func (b *DirectBackend) SendCustomerTextMessage(ctx context.Context, meta RequestMeta, conversationID string, input CustomerTextMessageInput) (ConversationMessage, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessage{}, err
	}
	message, err := b.sendCustomerTextMessage.Execute(ctx, identity, conversationaction.CustomerTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body,
	})
	if err != nil {
		return ConversationMessage{}, customerTextMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	slog.Info("成员客户文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"sender_identity_id", identity.OrganizationIdentity.ID,
	)
	return conversationMessageFromAction(message), nil
}

// StartDirectConversation 发起或打开企业成员内部单聊。
func (b *DirectBackend) StartDirectConversation(ctx context.Context, meta RequestMeta, input DirectConversationInput) (InboxConversation, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return InboxConversation{}, err
	}
	summary, err := b.startDirectConversation.Execute(ctx, identity, conversationaction.DirectConversationInput{
		TargetIdentityID: input.TargetIdentityID,
	})
	if err != nil {
		return InboxConversation{}, directConversationError(ctx, meta, err, identity.Organization.ID, input.TargetIdentityID, "start")
	}
	slog.Info("企业成员内部单聊已打开",
		"organization_id", identity.Organization.ID,
		"conversation_id", summary.ID,
		"target_identity_id", summary.PeerIdentityID,
	)
	return InboxConversation{
		ID: summary.ID, Type: ConversationTypeDirect,
		Direct: &DirectInboxConversation{
			PeerIdentityID: summary.PeerIdentityID, PeerName: summary.PeerName,
			Preview: summary.Preview, LastMessageAt: summary.LastMessageAt,
		},
	}, nil
}

// SendDirectTextMessage 发送内部单聊文本消息。
func (b *DirectBackend) SendDirectTextMessage(ctx context.Context, meta RequestMeta, conversationID string, input DirectTextMessageInput) (ConversationMessage, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessage{}, err
	}
	message, err := b.sendDirectTextMessage.Execute(ctx, identity, conversationaction.DirectTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body,
	})
	if err != nil {
		return ConversationMessage{}, directConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "send")
	}
	slog.Info("企业成员内部单聊文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"sender_identity_id", identity.OrganizationIdentity.ID,
	)
	return conversationMessageFromAction(message), nil
}

// ListConversationMessages 返回成员可见的会话消息。
func (b *DirectBackend) ListConversationMessages(ctx context.Context, meta RequestMeta, conversationID string, input ConversationMessageListInput) (ConversationMessageList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessageList{}, err
	}
	actionInput := conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID}
	if input.Before != "" && input.After != "" {
		return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"cursor": cervii18n.FieldMessageCursorInvalid})
	}
	if input.Before != "" {
		point, valid := decodeConversationMessageCursor(input.Before, conversationID)
		if !valid {
			return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"before": cervii18n.FieldMessageCursorInvalid})
		}
		actionInput.Before = &point
	}
	if input.After != "" {
		point, valid := decodeConversationMessageCursor(input.After, conversationID)
		if !valid {
			return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"after": cervii18n.FieldMessageCursorInvalid})
		}
		actionInput.After = &point
	}

	history, err := b.listConversationMessages.Execute(ctx, identity, actionInput)
	if err != nil {
		return ConversationMessageList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	result := ConversationMessageList{Messages: make([]ConversationMessage, 0, len(history.Messages))}
	for _, message := range history.Messages {
		result.Messages = append(result.Messages, conversationMessageFromAction(message))
	}
	if history.Before != nil {
		value := encodeConversationMessageCursor(conversationID, *history.Before)
		result.Before = &value
	}
	if history.After != nil {
		value := encodeConversationMessageCursor(conversationID, *history.After)
		result.After = &value
	}
	return result, nil
}

// conversationMessageFromAction 转换成员会话消息契约。
func conversationMessageFromAction(message conversationaction.ConversationMessage) ConversationMessage {
	var sender *ConversationMessageSender
	if message.Sender != nil {
		sender = &ConversationMessageSender{
			ChatSubjectID: message.Sender.ChatSubjectID,
			Kind:          ChatSubjectKind(message.Sender.Kind),
			SourceID:      message.Sender.SourceID,
			DisplayName:   message.Sender.DisplayName,
		}
	}
	var sessionStart *ConversationMessageSessionStart
	if message.SessionStart != nil {
		sessionStart = &ConversationMessageSessionStart{
			Sequence:  message.SessionStart.Sequence,
			StartedAt: message.SessionStart.StartedAt,
			Status:    ServiceSessionStatus(message.SessionStart.Status),
		}
	}
	return ConversationMessage{
		ID: message.ID, Type: MessageType(message.Type), Body: message.Body,
		OriginatedAt: message.OriginatedAt, CreatedAt: message.CreatedAt,
		Sender: sender, SessionStart: sessionStart,
	}
}

// directConversationError 转换内部单聊命令错误。
func directConversationError(ctx context.Context, meta RequestMeta, err error, organizationID, targetID, operation string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrDirectTargetNotFound) {
		return NotFoundError(meta, cervii18n.ErrorDirectTargetNotFound)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	var validationError *conversationaction.ValidationError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	var conflictError *conversationaction.ConflictError
	if errors.As(err, &conflictError) {
		return ConflictError(meta, cervii18n.ErrorDirectMessageConflict, conflictError.Reason)
	}
	slog.Warn("内部单聊命令失败", "organization_id", organizationID, "target_id", targetID, "operation", operation, "error", err)
	if operation == "start" {
		return FailedError(meta, cervii18n.ErrorDirectConversationStartFailed)
	}
	return FailedError(meta, cervii18n.ErrorDirectMessageSendFailed)
}

// encodeConversationMessageCursor 编码绑定会话的成员消息游标。
func encodeConversationMessageCursor(conversationID string, point conversationaction.MessageCursorPoint) string {
	return conversationID + "." + strconv.FormatInt(point.OriginatedAt.UnixNano(), 10) + "." + strconv.FormatInt(point.SourceOrder, 10) + "." + point.ID
}

// decodeConversationMessageCursor 解码并校验成员消息游标所属会话。
func decodeConversationMessageCursor(value, conversationID string) (conversationaction.MessageCursorPoint, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != conversationID || !common.ValidUUID(parts[3]) {
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

// conversationMessageError 转换成员消息读取错误。
func conversationMessageError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	var validationError *conversationaction.ValidationError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	slog.Warn("读取会话消息失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorConversationMessageListFailed)
}

// customerTextMessageError 转换成员客户消息发送错误。
func customerTextMessageError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	var validationError *conversationaction.ValidationError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	var conflictError *conversationaction.ConflictError
	if errors.As(err, &conflictError) {
		messageKey := cervii18n.ErrorCustomerMessageConflict
		switch conflictError.Reason {
		case conversationaction.ConflictReasonServiceSessionOwned:
			messageKey = cervii18n.ErrorServiceSessionOwned
		case conversationaction.ConflictReasonServiceSessionNotReplyable:
			messageKey = cervii18n.ErrorServiceSessionNotReplyable
		case conversationaction.ConflictReasonChannelOutboundUnsupported:
			messageKey = cervii18n.ErrorChannelOutboundUnsupported
		}
		return ConflictError(meta, messageKey, conflictError.Reason)
	}
	slog.Warn("发送成员客户消息失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorCustomerMessageSendFailed)
}

var conversationMessageValidationKeys = map[conversationaction.ValidationCode]cervii18n.Key{
	conversationaction.ValidationConversationIDInvalid:   cervii18n.FieldConversationIDInvalid,
	conversationaction.ValidationClientMessageIDInvalid:  cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationBodyRequired:            cervii18n.FieldMessageBodyRequired,
	conversationaction.ValidationBodyTooLong:             cervii18n.FieldMessageBodyTooLong,
	conversationaction.ValidationCursorInvalid:           cervii18n.FieldMessageCursorInvalid,
	conversationaction.ValidationTargetIdentityIDInvalid: cervii18n.FieldTargetIdentityIDInvalid,
}
