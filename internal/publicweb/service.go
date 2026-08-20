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
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// themeBlockStart 与 themeBlockEnd 标记嵌入脚本里由服务端替换的主题变量。
const (
	themeBlockStart = "/*CV_THEME_BLOCK*/"
	themeBlockEnd   = "/*END_CV_THEME_BLOCK*/"
)

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
	Placeholder    string
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
	theme := DefaultTheme()
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

// lookupWidgetTheme 读取渠道主题色；读失败时回退默认蓝并记日志。
func (s *EmbedService) lookupWidgetTheme(ctx context.Context, channelID string) Theme {
	if !common.ValidUUID(channelID) {
		slog.Warn("网站嵌入脚本渠道标识无效", "channel_id", channelID)
		return DefaultTheme()
	}
	channel, err := s.lookup(ctx, channelID)
	if errors.Is(err, channelaction.ErrNotFound) {
		slog.Warn("网站嵌入脚本渠道不存在", "channel_id", channelID)
		return DefaultTheme()
	}
	if err != nil {
		slog.Warn("读取网站嵌入脚本渠道失败", "channel_id", channelID, "error", err)
		return DefaultTheme()
	}
	return ParseTheme(channel.ThemeColor)
}

// scriptWithTheme 把计算后的主题变量写入脚本哨兵块。
func scriptWithTheme(theme Theme) []byte {
	start := bytes.Index(widgetScript, []byte(themeBlockStart))
	end := bytes.Index(widgetScript, []byte(themeBlockEnd))
	if start < 0 || end < 0 || end < start {
		slog.Error("网站嵌入脚本缺少主题哨兵")
		return widgetScript
	}
	end += len(themeBlockEnd)
	block := []byte(themeBlockStart + theme.HostCSS() + themeBlockEnd)
	script := make([]byte, 0, start+len(block)+len(widgetScript)-end)
	script = append(script, widgetScript[:start]...)
	script = append(script, block...)
	script = append(script, widgetScript[end:]...)
	return script
}

// writeChatPage 渲染公开聊天页。
func writeChatPage(writer http.ResponseWriter, request *http.Request, lookup Lookup, channelID string, entry string) {
	channel, err := lookup(request.Context(), channelID)
	if errors.Is(err, channelaction.ErrNotFound) {
		slog.Info("网站渠道聊天入口不存在", "channel_id", channelID, "entry", entry)
		_ = writePage(writer, notFoundView(request, entry), http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Warn("读取公开网站渠道失败", "channel_id", channelID, "entry", entry, "error", err)
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := writePage(writer, chatView(channel, entry), http.StatusOK); err != nil {
		return
	}
	slog.Info("打开网站渠道聊天页", "channel_id", channel.ID, "entry", entry, "title", channel.Title)
}

// writePage 写入公开聊天 HTML。
func writePage(writer http.ResponseWriter, page pageView, status int) error {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Security-Policy", "frame-ancestors *")
	// X-Frame-Options 无法按域名放行，用无效值避免上游补 SAMEORIGIN。
	writer.Header().Set("X-Frame-Options", "ALLOWALL")
	writer.WriteHeader(status)
	if err := pageTemplate.Execute(writer, page); err != nil {
		slog.Warn("渲染公开聊天页失败", "title", page.Title, "error", err)
		return err
	}
	return nil
}

// chatView 按渠道设置生成聊天页。
func chatView(channel *channelaction.PublicWebsiteChannel, entry string) pageView {
	english := channel.DefaultLocale == domain.LocaleEnglishUnitedStates
	page := baseView(entry, ParseTheme(channel.ThemeColor), english)
	page.Title = channel.Title
	page.Subtitle = channel.Subtitle
	page.Monogram = firstRunes(channel.Title, 1)
	page.AvatarInitials = firstRunes(channel.Title, 2)
	page.Greeting = channel.Greeting
	if english {
		page.Placeholder = "Type a message…"
		page.DemoReply = "This is a sample preview reply, shown only to demonstrate the bubble style."
		page.Lang = "en-US"
	} else {
		page.Placeholder = "输入消息…"
		page.DemoReply = "这是一条预览示例回复，仅用于查看气泡样式。"
		page.Lang = "zh-CN"
	}
	return page
}

// notFoundView 返回聊天入口不存在时的页面。
func notFoundView(request *http.Request, entry string) pageView {
	english := prefersEnglish(request.Header.Get("Accept-Language"))
	page := baseView(entry, DefaultTheme(), english)
	page.NotFound = true
	page.Monogram = "?"
	page.AvatarInitials = "?"
	if english {
		page.Lang = "en-US"
		page.Title = "Chat unavailable"
		page.EmptyMessage = "This chat link is not available."
	} else {
		page.Lang = "zh-CN"
		page.Title = "无法打开聊天"
		page.EmptyMessage = "这个聊天入口不可用。"
	}
	return page
}

// baseView 填充聊天页的共用字段。
func baseView(entry string, theme Theme, english bool) pageView {
	page := pageView{
		Shell:     entry,
		ShowClose: entry == "embed",
		ThemeCSS:  template.CSS(theme.RootCSS()),
		ChromeCSS: template.CSS(chromeCSS),
		ChatJS:    template.JS(chatJS),
	}
	if english {
		page.CloseLabel = "Close chat"
		page.AttachLabel = "Attach file"
		page.ImageLabel = "Add image"
		page.EmojiLabel = "Choose emoji"
	} else {
		page.CloseLabel = "关闭聊天"
		page.AttachLabel = "添加附件"
		page.ImageLabel = "添加图片"
		page.EmojiLabel = "选择表情"
	}
	return page
}

// firstRunes 取标题前若干字作为顶栏或头像字标。
func firstRunes(value string, count int) string {
	var out strings.Builder
	taken := 0
	for _, character := range strings.TrimSpace(value) {
		out.WriteString(strings.ToUpper(string(character)))
		taken++
		if taken >= count {
			break
		}
	}
	if out.Len() == 0 {
		return "?"
	}
	return out.String()
}

// prefersEnglish 判断请求是否优先使用英语。
func prefersEnglish(acceptLanguage string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(acceptLanguage)), "en")
}
