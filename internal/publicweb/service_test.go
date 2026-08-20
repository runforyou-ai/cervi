//go:build server

package publicweb

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestEmbedServiceServesWidgetScript 验证嵌入脚本按 JavaScript 返回。
func TestEmbedServiceServesWidgetScript(t *testing.T) {
	service := NewEmbedService(func(context.Context, string) (*channelaction.PublicWebsiteChannel, error) {
		t.Fatal("widget script should not look up a channel")
		return nil, nil
	})
	response := httptest.NewRecorder()
	service.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget.js", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	contentType := response.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/javascript") {
		t.Fatalf("content type = %q, want javascript", contentType)
	}
	body := response.Body.String()
	if !strings.Contains(body, `searchParams.get("id")`) {
		t.Fatalf("widget script missing id query reader: %s", body)
	}
}

// TestPublicChatPages 验证公开聊天页按渠道状态返回占位内容。
func TestPublicChatPages(t *testing.T) {
	channelID := "0191a2b3-c4d5-7890-abcd-ef1234567890"
	lookup := func(_ context.Context, id string) (*channelaction.PublicWebsiteChannel, error) {
		if id != channelID {
			return nil, channelaction.ErrNotFound
		}
		return &channelaction.PublicWebsiteChannel{
			ID:            channelID,
			Title:         "在线咨询",
			DefaultLocale: domain.LocaleChineseSimplified,
		}, nil
	}
	embed := NewEmbedService(lookup)
	chat := NewChatService(lookup)

	t.Run("embed frame", func(t *testing.T) {
		response := httptest.NewRecorder()
		embed.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget/"+channelID, nil))
		assertChatPage(t, response, http.StatusOK, "在线咨询")
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors *" {
			t.Fatalf("csp = %q", response.Header().Get("Content-Security-Policy"))
		}
	})

	t.Run("standalone chat", func(t *testing.T) {
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/"+channelID, nil))
		assertChatPage(t, response, http.StatusOK, "在线咨询")
	})

	t.Run("unknown channel", func(t *testing.T) {
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/0191a2b3-c4d5-7890-abcd-ef1234567891", nil))
		assertChatPage(t, response, http.StatusNotFound, "无法打开聊天")
	})

	t.Run("invalid id", func(t *testing.T) {
		response := httptest.NewRecorder()
		embed.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget/not-a-uuid", nil))
		assertChatPage(t, response, http.StatusNotFound, "无法打开聊天")
	})

	t.Run("english not found", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/not-a-uuid", nil)
		request.Header.Set("Accept-Language", "en-US")
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, request)
		assertChatPage(t, response, http.StatusNotFound, "Chat unavailable")
	})
}

// TestPublicChatLookupError 验证读取公开渠道失败时返回服务器错误。
func TestPublicChatLookupError(t *testing.T) {
	service := NewChatService(func(context.Context, string) (*channelaction.PublicWebsiteChannel, error) {
		return nil, errors.New("query failed")
	})
	response := httptest.NewRecorder()
	service.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/0191a2b3-c4d5-7890-abcd-ef1234567890", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func assertChatPage(t *testing.T, response *httptest.ResponseRecorder, status int, title string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d", response.Code, status)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), title) {
		t.Fatalf("body %q does not contain %q", body, title)
	}
}
