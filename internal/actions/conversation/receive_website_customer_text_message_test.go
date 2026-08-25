//go:build server

package conversation

import (
	"strings"
	"testing"
	"unicode/utf8"
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

// TestConversationTitleCollapsesWhitespaceAndLimitsRunes 验证会话标题压缩空白并限制字符数。
func TestConversationTitleCollapsesWhitespaceAndLimitsRunes(t *testing.T) {
	title := conversationTitle("  访客   问题\n" + strings.Repeat("鹿", 70))
	if !strings.HasPrefix(title, "访客 问题 ") || utf8.RuneCountInString(title) != 60 {
		t.Fatalf("conversation title = %q", title)
	}
}
