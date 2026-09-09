//go:build server

package conversation

import (
	"testing"
)

// TestNormalizeWebsiteMessageInput 验证网站消息正文和身份输入规范化。
func TestNormalizeWebsiteMessageInput(t *testing.T) {
	input := WebsiteCustomerTextMessageInput{
		ChannelID:       "0198ddee-c056-7bc5-a1d9-586f878ee966",
		ExternalID:      "web-session:0123456789abcdef0123456789abcdef",
		ClientMessageID: "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
		Body:            "  第一行\n第二行  ",
	}
	normalized, fields := normalizeWebsiteMessageInput(input)
	if len(fields) != 0 {
		t.Fatalf("validation fields = %#v", fields)
	}
	if normalized.Body != "第一行\n第二行" {
		t.Fatalf("normalized body = %q", normalized.Body)
	}
}

// TestValidWebsiteExternalIDRejectsNonHex 验证网站外部身份只接受十六进制令牌。
func TestValidWebsiteExternalIDRejectsNonHex(t *testing.T) {
	if validWebsiteExternalID("web-session:gggggggggggggggggggggggggggggggg") {
		t.Fatal("expected non-hex external ID to be rejected")
	}
}

// TestNormalizeWebsiteReplyTarget 验证引用编号格式和新会话的引用限制。
func TestNormalizeWebsiteReplyTarget(t *testing.T) {
	conversationID := "0198ddee-c056-7bc5-a1d9-586f878ee966"
	for _, input := range []WebsiteCustomerTextMessageInput{
		{ReplyToMessageID: "invalid", ConversationID: &conversationID},
		{ReplyToMessageID: conversationID},
	} {
		_, fields := normalizeWebsiteMessageInput(input)
		if fields["replyToMessageId"] != ValidationReplyToMessageIDInvalid {
			t.Fatalf("fields=%+v", fields)
		}
	}
	input, fields := normalizeWebsiteMessageInput(WebsiteCustomerTextMessageInput{ReplyToMessageID: "0198DDEE-C056-7BC5-A1D9-586F878EE966", ConversationID: &conversationID})
	if _, invalid := fields["replyToMessageId"]; invalid || input.ReplyToMessageID != conversationID {
		t.Fatalf("input=%+v fields=%+v", input, fields)
	}
}
