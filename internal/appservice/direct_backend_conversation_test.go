//go:build server

package appservice

import (
	"context"
	"errors"
	"testing"
	"time"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
)

// TestConversationMessageCursorRoundTrip 验证成员消息游标绑定会话并可稳定还原。
func TestConversationMessageCursorRoundTrip(t *testing.T) {
	const conversationID = "0198ddee-c056-7bc5-a1d9-586f878ee966"
	point := conversationaction.MessageCursorPoint{
		OriginatedAt: time.Date(2026, time.August, 29, 8, 30, 0, 123456789, time.UTC),
		ID:           "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
	}
	cursor := encodeConversationMessageCursor(conversationID, point)
	decoded, valid := decodeConversationMessageCursor(cursor, conversationID)
	if !valid || decoded.ID != point.ID || !decoded.OriginatedAt.Equal(point.OriginatedAt) {
		t.Fatalf("decoded cursor = %#v, valid = %v", decoded, valid)
	}
}

// TestConversationMessageCursorRejectsAnotherConversation 验证游标不能跨会话复用。
func TestConversationMessageCursorRejectsAnotherConversation(t *testing.T) {
	point := conversationaction.MessageCursorPoint{
		OriginatedAt: time.Now().UTC(),
		ID:           "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
	}
	cursor := encodeConversationMessageCursor("0198ddee-c056-7bc5-a1d9-586f878ee966", point)
	if _, valid := decodeConversationMessageCursor(cursor, "0198ddee-c056-7bc5-a1d9-586f878ee977"); valid {
		t.Fatal("expected cross-conversation cursor to be rejected")
	}
}

// TestCustomerTextMessageErrorMapsConflicts 验证成员回复冲突保留稳定原因。
func TestCustomerTextMessageErrorMapsConflicts(t *testing.T) {
	tests := []struct {
		reason  string
		message string
	}{
		{reason: conversationaction.ConflictReasonIdempotencyMismatch, message: "这条消息与之前的发送内容不一致。"},
		{reason: conversationaction.ConflictReasonServiceSessionOwned, message: "这条会话已由其他客服负责。"},
		{reason: conversationaction.ConflictReasonServiceSessionNotReplyable, message: "当前客服处理周期无法回复。"},
	}
	for _, test := range tests {
		err := customerTextMessageError(context.Background(), RequestMeta{Locale: LocaleChineseSimplified}, &conversationaction.ConflictError{Reason: test.reason}, "organization-1", "conversation-1")
		var apiError *Error
		if !errors.As(err, &apiError) || apiError.Kind != ErrorKindConflict || apiError.Reason != test.reason || apiError.Message != test.message {
			t.Fatalf("error = %#v", err)
		}
	}
}
