package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSendTextOutcomes 验证 Telegram 发送结果分类和纯文本请求。
func TestSendTextOutcomes(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		response string
		code     string
		retry    time.Duration
	}{
		{"success", 200, `{"ok":true,"result":{"message_id":42,"chat":{"id":123}}}`, "", 0},
		{"rate limit", 429, `{"ok":false,"error_code":429,"parameters":{"retry_after":7}}`, "rate_limited", 7 * time.Second},
		{"blocked", 403, `{"ok":false,"error_code":403}`, "recipient_unavailable", 0},
		{"token", 401, `{"ok":false,"error_code":401}`, "invalid_token", 0},
		{"rejected", 400, `{"ok":false,"error_code":400}`, "message_rejected", 0},
		{"ambiguous server error", 502, `{"ok":false,"error_code":502}`, "unknown_result", 0},
		{"invalid JSON", 200, `broken response`, "unknown_result", 0},
		{"wrong chat", 200, `{"ok":true,"result":{"message_id":42,"chat":{"id":999}}}`, "unknown_result", 0},
		{"missing message", 200, `{"ok":true,"result":{"chat":{"id":123}}}`, "unknown_result", 0},
		{"missing retry delay", 429, `{"ok":false,"error_code":429}`, "unknown_result", 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/sendMessage") || body["chat_id"] != float64(123) || body["text"] != "你好 *plain*" || len(body) != 2 {
					t.Errorf("unexpected request: %v", body)
				}
				w.WriteHeader(test.status)
				_, _ = fmt.Fprint(w, test.response)
			}))
			defer server.Close()
			id, err := NewClient(server.Client(), WithBaseURL(server.URL)).SendText(context.Background(), testBotToken, "123", "你好 *plain*")
			if test.code == "" {
				if err != nil || id != 42 {
					t.Fatalf("id=%d err=%v", id, err)
				}
				return
			}
			var failure *SendError
			if !errors.As(err, &failure) || failure.Code != test.code || failure.RetryAfter != test.retry {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(err.Error(), testBotToken) {
				t.Fatal("token leaked")
			}
		})
	}
}

// TestSendTextNetworkFailure 验证传输错误不会泄漏 Token 或被自动重试。
func TestSendTextNetworkFailure(t *testing.T) {
	client := NewClient(httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network error with %s", testBotToken)
	}))
	_, err := client.SendText(context.Background(), testBotToken, "123", "hello")
	var failure *SendError
	if !errors.As(err, &failure) || failure.Code != "unknown_result" || strings.Contains(err.Error(), testBotToken) {
		t.Fatalf("error=%v", err)
	}
}
