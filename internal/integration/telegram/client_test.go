package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

const testBotToken = "123456:test_token"

// TestGetMe 验证 getMe 请求和机器人身份解析。
func TestGetMe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/bot"+testBotToken+"/getMe" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "{}" {
			t.Fatalf("request body = %s, want {}", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true,"result":{"id":987654321,"is_bot":true,"first_name":"Cervi","last_name":"Support","username":"cervi_support_bot"}}`))
	}))
	defer server.Close()

	bot, err := NewClient(server.Client(), WithBaseURL(server.URL)).GetMe(context.Background(), testBotToken)
	if err != nil {
		t.Fatal(err)
	}
	if bot.ID != 987654321 || !bot.IsBot || bot.FirstName != "Cervi" || bot.LastName != "Support" || bot.Username != "cervi_support_bot" {
		t.Fatalf("unexpected bot: %#v", bot)
	}
}

// TestSetWebhook 验证注册参数固定限制为 my_chat_member 并丢弃积压 Update。
func TestSetWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/bot"+testBotToken+"/setWebhook" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		const expected = `{"url":"https://cervi.example/api/public/telegram-channels/channel-id/webhook","secret_token":"secret_token","allowed_updates":["my_chat_member"],"drop_pending_updates":true}`
		if string(body) != expected {
			t.Fatalf("request body = %s, want %s", body, expected)
		}
		_, _ = writer.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()

	err := NewClient(server.Client(), WithBaseURL(server.URL)).SetWebhook(
		context.Background(),
		testBotToken,
		Webhook{
			URL:    "https://cervi.example/api/public/telegram-channels/channel-id/webhook",
			Secret: "secret_token",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

// TestGetMeResponseFailures 验证 Telegram 响应异常会归类且不暴露原始响应。
func TestGetMeResponseFailures(t *testing.T) {
	tests := []struct {
		name string
		body string
		kind connectiontest.FailureKind
	}{
		{name: "not a bot", body: `{"ok":true,"result":{"id":1,"is_bot":false,"first_name":"User"}}`, kind: connectiontest.FailureProtocol},
		{name: "malformed JSON", body: `{`, kind: connectiontest.FailureProtocol},
		{name: "unauthorized", body: `{"ok":false,"error_code":401,"description":"token leaked in response"}`, kind: connectiontest.FailureUnauthorized},
		{name: "rate limited", body: `{"ok":false,"error_code":429}`, kind: connectiontest.FailureRateLimited},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()

			_, err := NewClient(server.Client(), WithBaseURL(server.URL)).GetMe(context.Background(), testBotToken)
			_, kind, classified := connectiontest.Details(err)
			if !classified || kind != test.kind {
				t.Fatalf("error = %v, kind = %q, want %q", err, kind, test.kind)
			}
			if strings.Contains(err.Error(), "token leaked") || strings.Contains(err.Error(), testBotToken) {
				t.Fatalf("error contains secret data: %v", err)
			}
		})
	}
}

// TestClientRejectsInvalidTokenBeforeRequest 验证非法 Token 不会发起网络请求。
func TestClientRejectsInvalidTokenBeforeRequest(t *testing.T) {
	called := false
	client := NewClient(httpDoerFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("unexpected request")
	}))
	_, err := client.GetMe(context.Background(), "invalid token")
	_, kind, classified := connectiontest.Details(err)
	if !classified || kind != connectiontest.FailureInvalidConfig {
		t.Fatalf("error = %v, kind = %q, want invalid_config", err, kind)
	}
	if called {
		t.Fatal("invalid token triggered a request")
	}
}

// TestTransportErrorDoesNotLeakTokenURL 验证传输错误剥离包含 Token 的请求地址。
func TestTransportErrorDoesNotLeakTokenURL(t *testing.T) {
	client := NewClient(httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return nil, &url.Error{
			Op:  "Post",
			URL: "https://api.telegram.org/bot" + testBotToken + "/getMe",
			Err: context.DeadlineExceeded,
		}
	}))
	_, err := client.GetMe(context.Background(), testBotToken)
	_, kind, classified := connectiontest.Details(err)
	if !classified || kind != connectiontest.FailureTimeout {
		t.Fatalf("error = %v, kind = %q, want timeout", err, kind)
	}
	if strings.Contains(err.Error(), testBotToken) || strings.Contains(err.Error(), "api.telegram.org/bot") {
		t.Fatalf("error contains Telegram request URL: %v", err)
	}
}

type httpDoerFunc func(*http.Request) (*http.Response, error)

// Do 调用测试函数。
func (f httpDoerFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}
