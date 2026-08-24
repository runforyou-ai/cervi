//go:build server

package publicweb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestAgentInitials 验证客服头像字标。
func TestAgentInitials(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"在线咨询", "在线"},
		{"support", "SU"},
		{"  ", "?"},
		{"", "?"},
	}
	for _, test := range cases {
		if got := agentInitials(test.value); got != test.want {
			t.Fatalf("agentInitials(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

// TestPreferredMessengerLocale 验证 Messenger 只把中文族浏览器识别为中文。
func TestPreferredMessengerLocale(t *testing.T) {
	cases := []struct {
		acceptLanguage string
		want           domain.Locale
	}{
		{"zh-CN,zh;q=0.9", domain.LocaleChineseSimplified},
		{"zh-TW", domain.LocaleChineseSimplified},
		{"en-US,en;q=0.9", domain.LocaleEnglishUnitedStates},
		{"ja-JP,ja;q=0.9", domain.LocaleEnglishUnitedStates},
		{"", domain.LocaleEnglishUnitedStates},
	}
	for _, test := range cases {
		if got := preferredMessengerLocale(test.acceptLanguage); got != test.want {
			t.Fatalf("preferredMessengerLocale(%q) = %q, want %q", test.acceptLanguage, got, test.want)
		}
	}
}

// TestComposerEmojis 验证访客 Messenger 的表情候选保持固定。
func TestComposerEmojis(t *testing.T) {
	var emojis []string
	if err := json.Unmarshal([]byte(composerEmojisJSON), &emojis); err != nil {
		t.Fatal(err)
	}
	if len(emojis) != 117 || emojis[0] != "😀" || emojis[53] != "❤️" || emojis[116] != "⚠️" {
		t.Fatalf("unexpected composer emojis: count=%d", len(emojis))
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(emojis, "\n"))))
	if digest != "3d5d8d7f47d67ace0e610ee0e5fcf68c5fb8fad95b8aa27a5b4bbecdc3e7d858" {
		t.Fatalf("composer emojis changed: sha256=%s", digest)
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
	if !strings.Contains(body, "width:400px;height:640px") || !strings.Contains(body, "100dvh - 144px") {
		t.Fatal("widget script missing default panel size or top spacing")
	}
	if !strings.Contains(body, "打开聊天") || !strings.Contains(body, "Open chat") {
		t.Fatal("widget script missing browser-language launcher copy")
	}
	if !strings.Contains(body, "cervi:toggle-expand") {
		t.Fatal("widget script missing expansion message contract")
	}
	previewResponse := httptest.NewRecorder()
	service.ServeHTTP(previewResponse, httptest.NewRequest(http.MethodGet, "/widget.js?preview=1", nil))
	if previewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("management preview widget script must not be cached")
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

// TestEmbedServiceRejectsUnknownChannel 验证不存在渠道不返回挂件脚本。
func TestEmbedServiceRejectsUnknownChannel(t *testing.T) {
	channelID := "0191a2b3-c4d5-7890-abcd-ef1234567890"
	service := NewEmbedService(func(context.Context, string) (*channelaction.PublicWebsiteChannel, error) {
		return nil, channelaction.ErrNotFound
	})
	response := httptest.NewRecorder()
	service.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget.js?id="+channelID, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "website channel not found") {
		t.Fatal("missing not found script response")
	}
}

// TestEmbedServiceReportsLookupError 验证渠道读取失败不返回挂件脚本。
func TestEmbedServiceReportsLookupError(t *testing.T) {
	channelID := "0191a2b3-c4d5-7890-abcd-ef1234567890"
	service := NewEmbedService(func(context.Context, string) (*channelaction.PublicWebsiteChannel, error) {
		return nil, errors.New("query failed")
	})
	response := httptest.NewRecorder()
	service.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widget.js?id="+channelID, nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "website channel unavailable") {
		t.Fatal("missing unavailable script response")
	}
}

// TestEmbedServiceRestrictsHost 验证安装脚本和聊天框只允许配置的网站加载。
func TestEmbedServiceRestrictsHost(t *testing.T) {
	channelID := "0191a2b3-c4d5-7890-abcd-ef1234567890"
	lookup := func(context.Context, string) (*channelaction.PublicWebsiteChannel, error) {
		return &channelaction.PublicWebsiteChannel{
			ID:                channelID,
			Title:             "在线咨询",
			ThemeColor:        "#2563EB",
			AllowedEmbedHosts: []string{"support.example.com"},
		}, nil
	}
	service := NewEmbedService(lookup)

	t.Run("allowed script", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/widget.js?id="+channelID, nil)
		request.Header.Set("Referer", "https://support.example.com/help")
		response := httptest.NewRecorder()
		service.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
	})

	t.Run("denied script", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/widget.js?id="+channelID, nil)
		request.Header.Set("Referer", "https://other.example.com/")
		response := httptest.NewRecorder()
		service.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d", response.Code)
		}
	})

	t.Run("allowed frame", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/widget/"+channelID, nil)
		request.Header.Set("Referer", "https://support.example.com/help")
		response := httptest.NewRecorder()
		service.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors support.example.com" {
			t.Fatalf("csp = %q", response.Header().Get("Content-Security-Policy"))
		}
	})

	t.Run("denied frame", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/widget/"+channelID, nil)
		request.Header.Set("Referer", "https://other.example.com/")
		response := httptest.NewRecorder()
		service.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d", response.Code)
		}
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors 'none'" {
			t.Fatalf("csp = %q", response.Header().Get("Content-Security-Policy"))
		}
	})

	t.Run("standalone chat", func(t *testing.T) {
		response := httptest.NewRecorder()
		NewChatService(lookup).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/"+channelID, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
	})
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
			DefaultLocale: domain.LocaleEnglishUnitedStates,
		}, nil
	}
	embed := NewEmbedService(lookup)
	chat := NewChatService(lookup)

	t.Run("embed frame", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/widget/"+channelID, nil)
		request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
		response := httptest.NewRecorder()
		embed.ServeHTTP(response, request)
		body := assertChatPage(t, response, http.StatusOK, "在线咨询")
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors *" {
			t.Fatalf("csp = %q", response.Header().Get("Content-Security-Policy"))
		}
		if response.Header().Get("Vary") != "Accept-Language" {
			t.Fatalf("vary = %q", response.Header().Get("Vary"))
		}
		assertMessengerPage(t, body, true)
		if !strings.Contains(body, "你好，我是客服。") {
			t.Fatal("missing greeting")
		}
		if !strings.Contains(body, `data-channel-greeting`) || !strings.Contains(body, `class="cv-presence-avatar`) {
			t.Fatal("missing greeting or service identity")
		}
		conversationHeader := pageElement(body, `<header class="cv-conversation-header">`, "</header>")
		if !strings.Contains(conversationHeader, "客服团队") || !strings.Contains(conversationHeader, "上次活跃：刚刚") {
			t.Fatal("conversation header missing localized service identity")
		}
		if strings.Contains(conversationHeader, "在线咨询") || strings.Contains(conversationHeader, "通常几分钟内回复") {
			t.Fatal("conversation header must not expose the channel title or subtitle")
		}
		if !strings.Contains(body, "谢谢，我们已经收到你的消息") {
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
		request := httptest.NewRequest(http.MethodGet, "/"+channelID, nil)
		request.Header.Set("Accept-Language", "zh-TW,zh;q=0.9")
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, request)
		body := assertChatPage(t, response, http.StatusOK, "在线咨询")
		assertMessengerPage(t, body, false)
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
				DefaultLocale: domain.LocaleChineseSimplified,
			}, nil
		}
		request := httptest.NewRequest(http.MethodGet, "/"+channelID, nil)
		request.Header.Set("Accept-Language", "ja-JP,ja;q=0.9")
		response := httptest.NewRecorder()
		NewChatService(englishLookup).ServeHTTP(response, request)
		body := assertChatPage(t, response, http.StatusOK, "Support")
		if !strings.Contains(body, `lang="en-US"`) {
			t.Fatal("missing english lang")
		}
		if !strings.Contains(body, "Thanks, we have your message.") {
			t.Fatal("missing english demo reply")
		}
		if !strings.Contains(body, "Choose emoji") {
			t.Fatal("missing english emoji label")
		}
		if !strings.Contains(body, `aria-label="Message"`) {
			t.Fatal("missing english message label")
		}
		conversationHeader := pageElement(body, `<header class="cv-conversation-header">`, "</header>")
		if !strings.Contains(conversationHeader, "Support team") || !strings.Contains(conversationHeader, "Last active just now") {
			t.Fatal("conversation header missing english service identity")
		}
	})

	t.Run("management preview", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/preview", nil)
		request.Header.Set("Accept-Language", "en-US")
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, request)
		body := assertChatPage(t, response, http.StatusOK, "Widget preview")
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors * wails:" {
			t.Fatalf("csp = %q", response.Header().Get("Content-Security-Policy"))
		}
		if response.Header().Get("Vary") != "Accept-Language" {
			t.Fatalf("vary = %q", response.Header().Get("Vary"))
		}
		if !strings.Contains(body, `class="cv-preview-site"`) || !strings.Contains(body, `/embed/widget.js?preview=1`) {
			t.Fatal("missing management widget preview host")
		}
		if !strings.Contains(body, `background: #f4f4f5`) || strings.Contains(body, "cv-preview-header") {
			t.Fatal("management widget preview must use a solid background")
		}

		request = httptest.NewRequest(http.MethodGet, "/preview/frame", nil)
		request.Header.Set("Accept-Language", "en-US")
		response = httptest.NewRecorder()
		embed.ServeHTTP(response, request)
		body = assertChatPage(t, response, http.StatusOK, "Support")
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors * wails:" {
			t.Fatalf("frame csp = %q", response.Header().Get("Content-Security-Policy"))
		}
		if !strings.Contains(body, `class="cv-preview"`) || !strings.Contains(body, `data-preview="true"`) {
			t.Fatal("missing management preview messenger")
		}
		if !strings.Contains(body, "How can we help?") || !strings.Contains(body, "Record voice message") {
			t.Fatal("missing preview messenger content")
		}
	})

	t.Run("unknown channel", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/0191a2b3-c4d5-7890-abcd-ef1234567891", nil)
		request.Header.Set("Accept-Language", "zh-CN")
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, request)
		body := assertChatPage(t, response, http.StatusNotFound, "无法打开聊天")
		if !strings.Contains(body, "这个聊天入口不可用。") {
			t.Fatal("missing not found copy")
		}
		if strings.Contains(body, `class="cv-composer"`) {
			t.Fatal("not found page must not include composer")
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/widget/not-a-uuid", nil)
		request.Header.Set("Accept-Language", "zh-CN")
		response := httptest.NewRecorder()
		embed.ServeHTTP(response, request)
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

// assertChatPage 验证聊天页响应并返回正文。
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

// pageElement 截取页面中指定的元素片段。
func pageElement(page string, startMarker string, endMarker string) string {
	start := strings.Index(page, startMarker)
	if start < 0 {
		return ""
	}
	end := strings.Index(page[start:], endMarker)
	if end < 0 {
		return ""
	}
	return page[start : start+end+len(endMarker)]
}

// assertMessengerPage 验证访客 Messenger 的页面契约。
func assertMessengerPage(t *testing.T, body string, embed bool) {
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
	if !strings.Contains(body, `aria-label="消息"`) || strings.Contains(body, `<label for="cv-input">`) {
		t.Fatal("composer must keep an accessible name without a visible label")
	}
	if strings.Contains(body, `placeholder=`) {
		t.Fatal("composer must not use a placeholder")
	}
	if !strings.Contains(body, `id="cv-attach"`) || !strings.Contains(body, `id="cv-emoji-toggle"`) || !strings.Contains(body, `id="cv-voice"`) || !strings.Contains(body, `id="cv-send"`) {
		t.Fatal("missing composer tools")
	}
	if !strings.Contains(body, `class="cv-emoji-tool"`) || !strings.Contains(body, "CERVI_COMPOSER_EMOJIS") {
		t.Fatal("emoji picker must be anchored to the shared composer emoji source")
	}
	if !strings.Contains(body, "bottom: calc(100% + 8px)") || !strings.Contains(body, "left: -41px") {
		t.Fatal("emoji picker must open above its composer button")
	}
	if !strings.Contains(body, "max-height: 160px") || !strings.Contains(body, "resize: none") {
		t.Fatal("composer textarea must grow with multiline content")
	}
	if !strings.Contains(body, `data-new-conversation`) || !strings.Contains(body, `data-resume-conversation`) {
		t.Fatal("missing new and recent conversation entry contracts")
	}
	if !strings.Contains(body, `<a class="cv-text-link" href="#help" data-route-target="help">查看全部</a>`) {
		t.Fatal("home help entry must be a link")
	}
	headingIndex := strings.Index(body, `class="cv-home-heading"`)
	recentIndex := strings.Index(body, `id="cv-home-recent"`)
	startIndex := strings.Index(body, `class="cv-start-card"`)
	helpIndex := strings.Index(body, `id="cv-home-help-title"`)
	if headingIndex < 0 || !(headingIndex < recentIndex && recentIndex < startIndex && startIndex < helpIndex) {
		t.Fatal("home content must show recent conversation before start chat and help")
	}
	homeRecent := pageElement(body, `<button class="cv-recent-card"`, "</button>")
	if strings.Contains(homeRecent, "最近对话") ||
		!strings.Contains(homeRecent, `data-channel-title`) ||
		!strings.Contains(homeRecent, `id="cv-home-recent-time"`) ||
		!strings.Contains(homeRecent, `id="cv-home-recent-preview"`) ||
		!strings.Contains(homeRecent, `id="cv-home-recent-unread-dot"`) {
		t.Fatal("home recent conversation must mirror the messages list content")
	}
	if embed && !strings.Contains(body, `class="cv-embed"`) {
		t.Fatal("missing embed shell class")
	}
	if embed && !strings.Contains(body, `id="cv-expand"`) {
		t.Fatal("embedded Messenger must include expansion control")
	}
	if !embed && !strings.Contains(body, `class="cv-link"`) {
		t.Fatal("missing standalone shell class")
	}
}
