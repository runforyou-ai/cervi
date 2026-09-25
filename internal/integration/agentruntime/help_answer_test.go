package agentruntime

import (
	"errors"
	"testing"
)

// TestParseHelpAnswer 验证回答解析、资料不足时的空回答和无效正文。
func TestParseHelpAnswer(t *testing.T) {
	answer, err := parseHelpAnswer("```json\n{\"answer\":\" 在设置页修改密码。 \"}\n```")
	if err != nil || answer != "在设置页修改密码。" {
		t.Fatalf("answer = %q, error = %v", answer, err)
	}
	answer, err = parseHelpAnswer(`{"answer":""}`)
	if err != nil || answer != "" {
		t.Fatalf("empty answer = %q, error = %v", answer, err)
	}
	for _, text := range []string{"直接回答", `{"reply":"一"}`, `{"answer":1}`} {
		if _, err := parseHelpAnswer(text); !errors.Is(err, errHelpAnswerInvalid) {
			t.Fatalf("parse %q error = %v", text, err)
		}
	}
}
