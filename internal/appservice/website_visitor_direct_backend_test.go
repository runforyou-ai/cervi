//go:build server

package appservice

import (
	"testing"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
)

// TestWebsiteMessageCursorRoundTrip 验证消息游标保留 Conversation 和稳定消息位置。
func TestWebsiteMessageCursorRoundTrip(t *testing.T) {
	const conversationID = "0198ddee-c056-7bc5-a1d9-586f878ee966"
	point := conversationaction.MessageCursorPoint{
		MessageSeq: 9007199254740993,
		ID:         "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
	}
	cursor := encodeConversationMessageCursor(conversationID, point)
	decoded, valid := decodeConversationMessageCursor(cursor, conversationID)
	if !valid || decoded.ID != point.ID || decoded.MessageSeq != point.MessageSeq {
		t.Fatalf("decoded cursor = %#v, valid = %t", decoded, valid)
	}
}

// TestWebsiteMessageCursorRejectsAnotherConversation 验证访客消息游标的会话归属校验。
func TestWebsiteMessageCursorRejectsAnotherConversation(t *testing.T) {
	point := conversationaction.MessageCursorPoint{
		MessageSeq: 9007199254740993,
		ID:         "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
	}
	cursor := encodeConversationMessageCursor("0198ddee-c056-7bc5-a1d9-586f878ee966", point)
	if _, valid := decodeConversationMessageCursor(cursor, "0198ddee-c056-7bc5-a1d9-586f878ee967"); valid {
		t.Fatal("expected cross-conversation cursor to be rejected")
	}
}
