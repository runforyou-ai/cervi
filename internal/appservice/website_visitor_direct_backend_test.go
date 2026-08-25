//go:build server

package appservice

import (
	"testing"
	"time"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
)

// TestWebsiteMessageCursorRoundTrip 验证消息游标保留 Conversation 和稳定消息位置。
func TestWebsiteMessageCursorRoundTrip(t *testing.T) {
	const conversationID = "0198ddee-c056-7bc5-a1d9-586f878ee966"
	point := conversationaction.MessageCursorPoint{
		OriginatedAt: time.Date(2026, 8, 25, 10, 20, 30, 123456789, time.UTC),
		ID:           "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
	}
	cursor := encodeWebsiteMessageCursor(conversationID, point)
	decoded, valid := decodeWebsiteMessageCursor(cursor, conversationID)
	if !valid || decoded.ID != point.ID || !decoded.OriginatedAt.Equal(point.OriginatedAt) {
		t.Fatalf("decoded cursor = %#v, valid = %t", decoded, valid)
	}
}

// TestWebsiteMessageCursorRejectsAnotherConversation 验证消息游标不能跨 Conversation 使用。
func TestWebsiteMessageCursorRejectsAnotherConversation(t *testing.T) {
	point := conversationaction.MessageCursorPoint{
		OriginatedAt: time.Date(2026, 8, 25, 10, 20, 30, 0, time.UTC),
		ID:           "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
	}
	cursor := encodeWebsiteMessageCursor("0198ddee-c056-7bc5-a1d9-586f878ee966", point)
	if _, valid := decodeWebsiteMessageCursor(cursor, "0198ddee-c056-7bc5-a1d9-586f878ee967"); valid {
		t.Fatal("expected cross-conversation cursor to be rejected")
	}
}
