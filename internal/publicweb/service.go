//go:build server

// Package publicweb 提供网站渠道的公开嵌入脚本和访客聊天页。
package publicweb

import (
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// Lookup 按渠道标识读取公开网站渠道。
type Lookup func(context.Context, string) (*channelaction.PublicWebsiteChannel, error)

type pageContent struct {
	Lang    string
	Title   string
	Message string
}

var pageTemplate = template.Must(template.New("chat").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{.Title}}</title>
    <style>
      :root { color-scheme: light; }
      * { box-sizing: border-box; }
      body {
        margin: 0;
        min-height: 100vh;
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
        background: #f8fafc;
        color: #0f172a;
      }
      main {
        width: min(28rem, calc(100% - 2rem));
        padding: 1.5rem;
        border: 1px solid #e2e8f0;
        border-radius: 1rem;
        background: #fff;
        box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
      }
      h1 { margin: 0 0 0.5rem; font-size: 1.25rem; font-weight: 600; }
      p { margin: 0; color: #64748b; line-height: 1.5; }
    </style>
  </head>
  <body>
    <main>
      <h1>{{.Title}}</h1>
      <p>{{.Message}}</p>
    </main>
  </body>
</html>`))

// EmbedService 提供 /embed/widget.js 和嵌入聊天框页面。
type EmbedService struct {
	lookup Lookup
}

// NewEmbedService 创建网站嵌入公开服务。
func NewEmbedService(lookup Lookup) *EmbedService {
	return &EmbedService{lookup: lookup}
}

// ServeHTTP 处理嵌入脚本和嵌入聊天框请求。
func (s *EmbedService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !allowPublicMethod(writer, request) {
		return
	}
	switch {
	case request.URL.Path == "/widget.js":
		writeWidgetScript(writer)
	case strings.HasPrefix(request.URL.Path, "/widget/"):
		writeChatPage(writer, request, s.lookup, strings.TrimPrefix(request.URL.Path, "/widget/"), "embed")
	default:
		http.NotFound(writer, request)
	}
}

// ChatService 提供独立聊天链接页面。
type ChatService struct {
	lookup Lookup
}

// NewChatService 创建独立聊天页公开服务。
func NewChatService(lookup Lookup) *ChatService {
	return &ChatService{lookup: lookup}
}

// ServeHTTP 处理独立聊天链接请求。
func (s *ChatService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !allowPublicMethod(writer, request) {
		return
	}
	writeChatPage(writer, request, s.lookup, strings.TrimPrefix(request.URL.Path, "/"), "link")
}

// allowPublicMethod 仅允许 GET 和 HEAD。
func allowPublicMethod(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		return true
	}
	writer.Header().Set("Allow", "GET, HEAD")
	http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	return false
}

// writeWidgetScript 返回网站嵌入脚本。
func writeWidgetScript(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	writer.Header().Set("Cache-Control", "public, max-age=300")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := writer.Write(widgetScript); err != nil {
		slog.Warn("写入网站嵌入脚本失败", "error", err)
	}
}

// writeChatPage 渲染公开聊天占位页。
func writeChatPage(writer http.ResponseWriter, request *http.Request, lookup Lookup, channelID string, entry string) {
	channel, err := lookup(request.Context(), channelID)
	if errors.Is(err, channelaction.ErrNotFound) {
		_ = writePlaceholder(writer, notFoundPage(request), http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Warn("读取公开网站渠道失败", "channel_id", channelID, "entry", entry, "error", err)
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := writePlaceholder(writer, chatPage(channel), http.StatusOK); err != nil {
		return
	}
	slog.Info("打开网站渠道聊天页", "channel_id", channel.ID, "entry", entry)
}

// writePlaceholder 写入公开聊天 HTML。
func writePlaceholder(writer http.ResponseWriter, page pageContent, status int) error {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Security-Policy", "frame-ancestors *")
	// X-Frame-Options 无法按域名放行，用无效值避免上游补 SAMEORIGIN。
	writer.Header().Set("X-Frame-Options", "ALLOWALL")
	writer.WriteHeader(status)
	if err := pageTemplate.Execute(writer, page); err != nil {
		slog.Warn("渲染公开聊天页失败", "error", err)
		return err
	}
	return nil
}

// chatPage 按渠道默认语言生成占位内容。
func chatPage(channel *channelaction.PublicWebsiteChannel) pageContent {
	if channel.DefaultLocale == domain.LocaleEnglishUnitedStates {
		return pageContent{
			Lang:    "en-US",
			Title:   channel.Title,
			Message: "Visitors will be able to start a conversation here.",
		}
	}
	return pageContent{
		Lang:    "zh-CN",
		Title:   channel.Title,
		Message: "访客可以在这里开始咨询。",
	}
}

// notFoundPage 返回聊天入口不存在时的页面。
func notFoundPage(request *http.Request) pageContent {
	if prefersEnglish(request.Header.Get("Accept-Language")) {
		return pageContent{
			Lang:    "en-US",
			Title:   "Chat unavailable",
			Message: "This chat link is not available.",
		}
	}
	return pageContent{
		Lang:    "zh-CN",
		Title:   "无法打开聊天",
		Message: "这个聊天入口不可用。",
	}
}

// prefersEnglish 判断请求是否优先使用英语。
func prefersEnglish(acceptLanguage string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(acceptLanguage)), "en")
}
