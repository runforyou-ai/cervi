//go:build server

// Package publicweb 提供网站渠道的公开嵌入脚本和访客聊天页。
package publicweb

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const themePlaceholder = "/*CV_THEME*/"

// Lookup 按渠道标识读取公开网站渠道。
type Lookup func(context.Context, string) (*channelaction.PublicWebsiteChannel, error)

type pageView struct {
	Lang           string
	Title          string
	Subtitle       string
	Monogram       string
	AvatarInitials string
	Greeting       string
	DemoReply      string
	MessageLabel   string
	EmptyMessage   string
	CloseLabel     string
	AttachLabel    string
	ImageLabel     string
	EmojiLabel     string
	Shell          string
	NotFound       bool
	ShowClose      bool
	ThemeCSS       template.CSS
	ChromeCSS      template.CSS
	ChatJS         template.JS
}

var pageTemplate = template.Must(template.New("chat").Parse(pageHTML))

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
		s.writeWidgetScript(writer, request)
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

// writeWidgetScript 返回按渠道内联主题色的网站嵌入脚本。
func (s *EmbedService) writeWidgetScript(writer http.ResponseWriter, request *http.Request) {
	theme := defaultTheme()
	channelID := strings.TrimSpace(request.URL.Query().Get("id"))
	if channelID != "" {
		theme = s.lookupWidgetTheme(request.Context(), channelID)
	}
	writer.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	writer.Header().Set("Cache-Control", "public, max-age=300")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := writer.Write(scriptWithTheme(theme)); err != nil {
		slog.Warn("写入网站嵌入脚本失败", "channel_id", channelID, "error", err)
	}
}

// lookupWidgetTheme 读取挂件主题。
func (s *EmbedService) lookupWidgetTheme(ctx context.Context, channelID string) theme {
	channel, err := s.lookup(ctx, channelID)
	if errors.Is(err, channelaction.ErrNotFound) {
		slog.Info("网站嵌入脚本渠道不存在", "channel_id", channelID)
		return defaultTheme()
	}
	if err != nil {
		slog.Warn("读取网站嵌入脚本渠道失败", "channel_id", channelID, "error", err)
		return defaultTheme()
	}
	return parseTheme(channel.ThemeColor)
}

// scriptWithTheme 写入挂件主题变量。
func scriptWithTheme(theme theme) []byte {
	return bytes.Replace(widgetScript, []byte(themePlaceholder), []byte(theme.hostCSS()), 1)
}

// writeChatPage 渲染公开聊天页。
func writeChatPage(writer http.ResponseWriter, request *http.Request, lookup Lookup, channelID string, entry string) {
	channel, err := lookup(request.Context(), channelID)
	if errors.Is(err, channelaction.ErrNotFound) {
		if err := writePage(writer, notFoundView(request, entry), http.StatusNotFound); err != nil {
			slog.Warn("写入网站渠道不可用页面失败", "channel_id", channelID, "entry", entry, "error", err)
			return
		}
		slog.Info("网站渠道聊天入口不存在", "channel_id", channelID, "entry", entry)
		return
	}
	if err != nil {
		slog.Warn("读取公开网站渠道失败", "channel_id", channelID, "entry", entry, "error", err)
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := writePage(writer, chatView(channel, entry), http.StatusOK); err != nil {
		slog.Warn("写入网站渠道聊天页失败", "channel_id", channel.ID, "entry", entry, "error", err)
		return
	}
	slog.Info("打开网站渠道聊天页", "channel_id", channel.ID, "entry", entry)
}

// writePage 写入公开聊天 HTML。
func writePage(writer http.ResponseWriter, page pageView, status int) error {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Security-Policy", "frame-ancestors *")
	writer.WriteHeader(status)
	return pageTemplate.Execute(writer, page)
}

// chatView 按渠道设置生成聊天页。
func chatView(channel *channelaction.PublicWebsiteChannel, entry string) pageView {
	english := channel.DefaultLocale == domain.LocaleEnglishUnitedStates
	page := baseView(entry, parseTheme(channel.ThemeColor), english)
	page.Title = channel.Title
	page.Subtitle = channel.Subtitle
	page.Monogram = firstRunes(channel.Title, 1)
	page.AvatarInitials = firstRunes(channel.Title, 2)
	page.Greeting = channel.Greeting
	if english {
		page.DemoReply = "This is a sample reply."
	} else {
		page.DemoReply = "这是一条示例回复。"
	}
	return page
}

// notFoundView 返回聊天入口不存在时的页面。
func notFoundView(request *http.Request, entry string) pageView {
	english := prefersEnglish(request.Header.Get("Accept-Language"))
	page := baseView(entry, defaultTheme(), english)
	page.NotFound = true
	page.Monogram = "?"
	page.AvatarInitials = "?"
	if english {
		page.Title = "Chat unavailable"
		page.EmptyMessage = "This chat link is not available."
	} else {
		page.Title = "无法打开聊天"
		page.EmptyMessage = "这个聊天入口不可用。"
	}
	return page
}

// baseView 填充聊天页共用内容。
func baseView(entry string, theme theme, english bool) pageView {
	page := pageView{
		Shell:     entry,
		ShowClose: entry == "embed",
		ThemeCSS:  template.CSS(theme.rootCSS()),
		ChromeCSS: template.CSS(chromeCSS),
		ChatJS:    template.JS(chatJS),
	}
	if english {
		page.Lang = "en-US"
		page.MessageLabel = "Message"
		page.CloseLabel = "Close chat"
		page.AttachLabel = "Attach file"
		page.ImageLabel = "Add image"
		page.EmojiLabel = "Choose emoji"
	} else {
		page.Lang = "zh-CN"
		page.MessageLabel = "消息"
		page.CloseLabel = "关闭聊天"
		page.AttachLabel = "添加附件"
		page.ImageLabel = "添加图片"
		page.EmojiLabel = "选择表情"
	}
	return page
}

// firstRunes 返回标题开头的字标。
func firstRunes(value string, count int) string {
	characters := []rune(strings.ToUpper(strings.TrimSpace(value)))
	if len(characters) == 0 {
		return "?"
	}
	if len(characters) > count {
		characters = characters[:count]
	}
	return string(characters)
}

// prefersEnglish 判断请求是否优先使用英语。
func prefersEnglish(acceptLanguage string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(acceptLanguage)), "en")
}
