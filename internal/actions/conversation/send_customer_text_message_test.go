//go:build server

package conversation

import (
	"strings"
	"testing"
)

// TestNormalizeCustomerTextMessageInput 验证成员消息输入按 Unicode 字符校验。
func TestNormalizeCustomerTextMessageInput(t *testing.T) {
	input := CustomerTextMessageInput{
		ConversationID:   "0198DDEE-C056-7BC5-A1D9-586F878EE966",
		ClientMessageID:  "0198DDF0-A234-7F01-8D99-E3E0AF0F5F65",
		Body:             "  回复客户  ",
		ReplyToMessageID: "0198DDF0-A234-7F01-8D99-E3E0AF0F5F66",
	}
	normalized, fields := normalizeCustomerTextMessageInput(input)
	if len(fields) != 0 || normalized.ReplyToMessageID != strings.ToLower(input.ReplyToMessageID) || normalized.Body != "回复客户" || normalized.ConversationID != strings.ToLower(input.ConversationID) || normalized.ClientMessageID != strings.ToLower(input.ClientMessageID) {
		t.Fatalf("normalized = %#v, fields = %#v", normalized, fields)
	}

	input.Body = strings.Repeat("鹿", 4001)
	_, fields = normalizeCustomerTextMessageInput(input)
	if fields["body"] != ValidationBodyTooLong {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestCustomerReplyInputValidation 验证引用目标只接受空值或有效 UUID。
func TestCustomerReplyInputValidation(t *testing.T) {
	for _, target := range []string{"", "invalid", " 0198ddf0-a234-7f01-8d99-e3e0af0f5f66 "} {
		_, fields := normalizeCustomerTextMessageInput(CustomerTextMessageInput{ReplyToMessageID: target})
		if (fields["replyToMessageId"] == ValidationReplyToMessageIDInvalid) != (target == "invalid") {
			t.Fatalf("target=%q fields=%+v", target, fields)
		}
	}
}
