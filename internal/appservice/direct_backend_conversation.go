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

// ListConversationMessages 返回成员可见的客户会话消息。
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
		var sender *ConversationMessageSender
		if message.Sender != nil {
			sender = &ConversationMessageSender{
				ChatSubjectID: message.Sender.ChatSubjectID,
				Kind:          ChatSubjectKind(message.Sender.Kind),
				DisplayName:   message.Sender.DisplayName,
			}
		}
		result.Messages = append(result.Messages, ConversationMessage{
			ID: message.ID, Type: MessageType(message.Type), Body: message.Body,
			OriginatedAt: message.OriginatedAt, CreatedAt: message.CreatedAt, Sender: sender,
		})
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

// encodeConversationMessageCursor 编码绑定会话的成员消息游标。
func encodeConversationMessageCursor(conversationID string, point conversationaction.MessageCursorPoint) string {
	return conversationID + "." + strconv.FormatInt(point.OriginatedAt.UnixNano(), 10) + "." + point.ID
}

// decodeConversationMessageCursor 解码并校验成员消息游标所属会话。
func decodeConversationMessageCursor(value, conversationID string) (conversationaction.MessageCursorPoint, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != conversationID || !common.ValidUUID(parts[2]) {
		return conversationaction.MessageCursorPoint{}, false
	}
	originatedAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || originatedAt <= 0 {
		return conversationaction.MessageCursorPoint{}, false
	}
	return conversationaction.MessageCursorPoint{OriginatedAt: time.Unix(0, originatedAt).UTC(), ID: parts[2]}, true
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

var conversationMessageValidationKeys = map[conversationaction.ValidationCode]cervii18n.Key{
	conversationaction.ValidationConversationIDInvalid: cervii18n.FieldConversationIDInvalid,
	conversationaction.ValidationCursorInvalid:         cervii18n.FieldMessageCursorInvalid,
}
