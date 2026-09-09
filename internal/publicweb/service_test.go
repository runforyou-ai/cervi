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

// TestChatServiceMarkdownAssets 验证企业服务端的正文资源路由和缓存策略。
func TestChatServiceMarkdownAssets(t *testing.T) {
	service := NewChatService(func(context.Context, string) (*channelaction.PublicWebsiteChannel, error) {
		t.Fatal("markdown assets must not look up a channel")
		return nil, nil
	})
	for _, name := range []string{"markdown.js", "markdown.css"} {
		response := httptest.NewRecorder()
		service.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/"+name, nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("asset %s: status=%d bytes=%d", name, response.Code, response.Body.Len())
		}
		if response.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("asset %s must revalidate after deployment", name)
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

// TestEmbedServiceServesWidgetScript 验证嵌入脚本的响应类型和预览缓存策略。
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
	previewResponse := httptest.NewRecorder()
	service.ServeHTTP(previewResponse, httptest.NewRequest(http.MethodGet, "/widget.js?preview=1", nil))
	if previewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("management preview widget script must not be cached")
	}
}

// TestEmbedServiceRejectsUnknownChannel 验证未知渠道返回未找到响应。
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

// TestEmbedServiceReportsLookupError 验证渠道读取失败返回服务错误。
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
		if !strings.Contains(body, "你好，我是客服。") {
			t.Fatal("missing greeting")
		}
		if !strings.Contains(body, "通常几分钟内回复") {
			t.Fatal("missing subtitle")
		}
	})

	t.Run("standalone chat", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/"+channelID, nil)
		request.Header.Set("Accept-Language", "zh-TW,zh;q=0.9")
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, request)
		assertChatPage(t, response, http.StatusOK, "在线咨询")
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
	})

	t.Run("management preview", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/preview", nil)
		request.Header.Set("Accept-Language", "en-US")
		response := httptest.NewRecorder()
		chat.ServeHTTP(response, request)
		assertChatPage(t, response, http.StatusOK, "Widget preview")
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors * wails:" {
			t.Fatalf("csp = %q", response.Header().Get("Content-Security-Policy"))
		}
		if response.Header().Get("Vary") != "Accept-Language" {
			t.Fatalf("vary = %q", response.Header().Get("Vary"))
		}

		request = httptest.NewRequest(http.MethodGet, "/preview/frame", nil)
		request.Header.Set("Accept-Language", "en-US")
		response = httptest.NewRecorder()
		embed.ServeHTTP(response, request)
		assertChatPage(t, response, http.StatusOK, "Support")
		if response.Header().Get("Content-Security-Policy") != "frame-ancestors * wails:" {
			t.Fatalf("frame csp = %q", response.Header().Get("Content-Security-Policy"))
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
