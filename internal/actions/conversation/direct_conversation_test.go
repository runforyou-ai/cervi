//go:build server

package conversation

import (
	"strings"
	"testing"
)

// TestNormalizeDirectIdentityPair 验证双方发起得到同一规范化身份对。
func TestNormalizeDirectIdentityPair(t *testing.T) {
	firstIdentityID := "0198ddf0-a234-7f01-8d99-e3e0af0f5f65"
	secondIdentityID := "0198ddf0-a234-7f01-8d99-e3e0af0f5f66"
	forwardFirst, forwardSecond := normalizeDirectIdentityPair(firstIdentityID, secondIdentityID)
	reverseFirst, reverseSecond := normalizeDirectIdentityPair(secondIdentityID, firstIdentityID)
	if forwardFirst != firstIdentityID || forwardSecond != secondIdentityID || reverseFirst != firstIdentityID || reverseSecond != secondIdentityID {
		t.Fatalf("normalized pairs = (%q, %q) and (%q, %q)", forwardFirst, forwardSecond, reverseFirst, reverseSecond)
	}
}

// TestNormalizeDirectTextMessageInput 验证内部单聊文本消息输入归一化。
func TestNormalizeDirectTextMessageInput(t *testing.T) {
	input := DirectTextMessageInput{
		ConversationID:  "0198DDEE-C056-7BC5-A1D9-586F878EE966",
		ClientMessageID: "0198DDF0-A234-7F01-8D99-E3E0AF0F5F65",
		Body:            "  你好，同事  ",
	}
	normalized, fields := normalizeDirectTextMessageInput(input)
	if len(fields) != 0 || normalized.Body != "你好，同事" || normalized.ConversationID != strings.ToLower(input.ConversationID) || normalized.ClientMessageID != strings.ToLower(input.ClientMessageID) {
		t.Fatalf("normalized = %#v, fields = %#v", normalized, fields)
	}
}
