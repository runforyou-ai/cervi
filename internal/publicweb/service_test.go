//go:build server

package publicweb

import (
	"bytes"
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

// TestFirstRunes 验证聊天页字标。
func TestFirstRunes(t *testing.T) {
	cases := []struct {
		value string
		count int
		want  string
	}{
		{"在线咨询", 1, "在"},
		{"在线咨询", 2, "在线"},
		{"support", 2, "SU"},
		{"  ", 1, "?"},
		{"", 2, "?"},
	}
	for _, test := range cases {
		if got := firstRunes(test.value, test.count); got != test.want {
			t.Fatalf("firstRunes(%q, %d) = %q, want %q", test.value, test.count, got, test.want)
		}
	}
}

// TestWidgetScriptHasThemePlaceholder 验证挂件主题占位符。
func TestWidgetScriptHasThemePlaceholder(t *testing.T) {
	if bytes.Count(widgetScript, []byte(themePlaceholder)) != 1 {
		t.Fatal("widget.js must contain one theme placeholder")
	}
}

// TestEmbedServiceServesWidgetScript 验证嵌入脚本按 JavaScript 返回默认主题。
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
	if strings.Contains(body, themePlaceholder) {
		t.Fatal("widget script contains unresolved theme placeholder")
	}
	if !strings.Contains(body, `searchParams.get("id")`) {
		t.Fatalf("widget script missing id query reader: %s", body)
	}
	if !strings.Contains(body, "--cv-theme:#2563EB") {
		t.Fatalf("widget script missing default theme: %s", body)
	}
}

// TestEmbedServiceInlinesChannelTheme 验证带渠道标识的脚本内联该渠道主题色。
func TestEmbedServiceInlinesChannelTheme(t *testing.T) {
	channelID := "0191a2b3-c4d5-7890-abcd-ef1234567890"
	service := NewEmbedService(func(_ context.Context, id string) (*channelaction.PublicWebsiteChannel, error) {
		if id != channelID {
			t.Fatalf("lookup id = %q", id)
		}
		return &channelaction.PublicWebsiteChannel{
			ID:         channelID,
			Title:      "在线咨询",
			ThemeColor: "#EA580C",
		}, nil
	})
	response := httptest.NewRecorder()
	service.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget.js?id="+channelID, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "--cv-theme:#EA580C") {
		t.Fatalf("missing channel theme: %s", body)
	}
	if !strings.Contains(body, "--cv-on-theme:#1C1917") {
		t.Fatalf("missing on-theme: %s", body)
	}
}

// TestEmbedServiceUnknownChannelUsesDefaultTheme 验证不存在渠道使用默认主题。
func TestEmbedServiceUnknownChannelUsesDefaultTheme(t *testing.T) {
	channelID := "0191a2b3-c4d5-7890-abcd-ef1234567890"
	service := NewEmbedService(func(context.Context, string) (*channelaction.PublicWebsiteChannel, error) {
		return nil, channelaction.ErrNotFound
	})
	response := httptest.NewRecorder()
	service.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget.js?id="+channelID, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "--cv-theme:#2563EB") {
		t.Fatal("unknown channel should use default theme")
	}
}

// TestEmbedServiceLookupErrorUsesDefaultTheme 验证读取失败时使用默认主题。
func TestEmbedServiceLookupErrorUsesDefaultTheme(t *testing.T) {
	channelID := "0191a2b3-c4d5-7890-abcd-ef1234567890"
	service := NewEmbedService(func(context.Context, string) (*channelaction.PublicWebsiteChannel, error) {
		return nil, errors.New("query failed")
	})
	response := httptest.NewRecorder()
	service.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget.js?id="+channelID, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "--cv-theme:#2563EB") {
		t.Fatal("lookup error should use default theme")
	}
}

// TestPublicChatPages 验证公开聊天页渲染访客界面。
func TestPublicChatPages(t *testing.T) {
	channelID := "0191a2b3-c4d5-7890-abcd-ef1234567890"
	lookup := func(_ context.Context, id string) (*channelaction.PublicWebsiteChannel, error) {
		if id != channelID {
			return nil, channelaction.ErrNotFound
		}
		return &channelaction.PublicWebsiteChannel{
			ID:            channelID,
			Title:         "在线咨询",
			Subtitle:      "通常几分钟内回复",
			Greeting:      "你好，我是客服。",
			ThemeColor:    "#2563EB",
			DefaultLocale: domain.LocaleChineseSimplified,
		}, nil
	}
	embed := NewEmbedService(lookup)
	chat := NewChatService(lookup)

	t.Run("embed frame", func(t *testing.T) {
		response := httptest.NewRecorder()
		embed.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget/"+channelID, nil))
		body := assertChatPage(t, response, http.StatusOK, "在线咨询")
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors *" {
			t.Fatalf("csp = %q", response.Header().Get("Content-Security-Policy"))
		}
		assertChrome(t, body, true)
		if !strings.Contains(body, "你好，我是客服。") {
			t.Fatal("missing greeting")
		}
		if !strings.Contains(body, `class="cv-avatar cv-avatar-assistant"`) || !strings.Contains(body, `class="cv-sender"`) {
			t.Fatal("missing greeting avatar or sender")
		}
		if !strings.Contains(body, "这是一条示例回复。") {
			t.Fatal("missing demo assistant reply copy")
		}
		if !strings.Contains(body, "通常几分钟内回复") {
			t.Fatal("missing subtitle")
		}
		if !strings.Contains(body, `class="cv-close"`) {
			t.Fatal("embed chrome must include close control")
		}
	})

	t.Run("standalone chat", func(t *testing.T) {
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/"+channelID, nil))
		body := assertChatPage(t, response, http.StatusOK, "在线咨询")
		assertChrome(t, body, false)
		if strings.Contains(body, `class="cv-close"`) {
			t.Fatal("standalone chrome must not include widget close")
		}
	})

	t.Run("english chat", func(t *testing.T) {
		englishLookup := func(_ context.Context, id string) (*channelaction.PublicWebsiteChannel, error) {
			if id != channelID {
				return nil, channelaction.ErrNotFound
			}
			return &channelaction.PublicWebsiteChannel{
				ID:            channelID,
				Title:         "Support",
				Greeting:      "Hello.",
				ThemeColor:    "#2563EB",
				DefaultLocale: domain.LocaleEnglishUnitedStates,
			}, nil
		}
		response := httptest.NewRecorder()
		NewChatService(englishLookup).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/"+channelID, nil))
		body := assertChatPage(t, response, http.StatusOK, "Support")
		if !strings.Contains(body, `lang="en-US"`) {
			t.Fatal("missing english lang")
		}
		if !strings.Contains(body, "This is a sample reply.") {
			t.Fatal("missing english demo reply")
		}
		if !strings.Contains(body, "Choose emoji") {
			t.Fatal("missing english emoji label")
		}
		if !strings.Contains(body, `for="cv-input">Message</label>`) {
			t.Fatal("missing english message label")
		}
	})

	t.Run("unknown channel", func(t *testing.T) {
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/0191a2b3-c4d5-7890-abcd-ef1234567891", nil))
		body := assertChatPage(t, response, http.StatusNotFound, "无法打开聊天")
		if !strings.Contains(body, "这个聊天入口不可用。") {
			t.Fatal("missing not found copy")
		}
		if strings.Contains(body, `class="cv-composer"`) {
			t.Fatal("not found page must not include composer")
		}
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

func assertChatPage(t *testing.T, response *httptest.ResponseRecorder, status int, title string) string {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d", response.Code, status)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, title) {
		t.Fatalf("body %q does not contain %q", text, title)
	}
	return text
}

func assertChrome(t *testing.T, body string, embed bool) {
	t.Helper()
	if !strings.Contains(body, "--cv-theme:") {
		t.Fatal("missing theme variables")
	}
	if !strings.Contains(body, "color: var(--cv-on-theme)") {
		t.Fatal("missing visitor bubble contrast color")
	}
	if !strings.Contains(body, `id="cv-input"`) || !strings.Contains(body, "<textarea") {
		t.Fatal("missing composer textarea")
	}
	if !strings.Contains(body, `for="cv-input">消息</label>`) {
		t.Fatal("missing composer label")
	}
	if strings.Contains(body, `placeholder=`) {
		t.Fatal("composer must not use a placeholder")
	}
	if !strings.Contains(body, `id="cv-attach"`) || !strings.Contains(body, `id="cv-image"`) || !strings.Contains(body, `id="cv-emoji-toggle"`) {
		t.Fatal("missing composer tools")
	}
	if !strings.Contains(body, "sendMessage") || !strings.Contains(body, "appendVisitorMessage") {
		t.Fatal("missing client demo script")
	}
	if !strings.Contains(body, "appendAssistantMessage") || !strings.Contains(body, "scheduleDemoReply") {
		t.Fatal("missing demo assistant reply")
	}
	if !strings.Contains(body, "toLocaleTimeString") || !strings.Contains(body, "cv-time") {
		t.Fatal("missing message clock")
	}
	if !strings.Contains(body, "pastedImageFiles") {
		t.Fatal("missing paste image handler")
	}
	if !strings.Contains(body, "cv-avatar-visitor") || !strings.Contains(body, "cv-avatar-assistant") {
		t.Fatal("missing visitor or assistant avatar")
	}
	if strings.Contains(body, "selectionStart ||") || strings.Contains(body, "selectionEnd ||") {
		t.Fatal("emoji insertion must preserve selection index zero")
	}
	if embed && !strings.Contains(body, `class="cv-embed"`) {
		t.Fatal("missing embed shell class")
	}
	if !embed && !strings.Contains(body, `class="cv-link"`) {
		t.Fatal("missing standalone shell class")
	}
}
