//go:build server

// Package publicweb 提供网站渠道的公开嵌入脚本和访客聊天页。
package publicweb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/common/embedhost"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

const themePlaceholder = "/*CV_THEME*/"

// copyPlaceholder 是嵌入脚本中挂件文案的注入位置。
const copyPlaceholder = "/*CV_COPY*/ null"

// Lookup 按渠道标识读取公开网站渠道。
type Lookup func(context.Context, string) (*channelaction.PublicWebsiteChannel, error)

type pageView struct {
	Lang               string
	ChannelID          string
	Title              string
	TitleInitials      string
	Greeting           string
	HomeLinks          []domain.WebsiteHomeLink
	EmptyMessage       string
	Shell              string
	NotFound           bool
	ShowWidgetControls bool
	Preview            bool
	Copy               map[string]string
	ThemeCSS           template.CSS
	MessengerCSS       template.CSS
	ComposerEmojis     template.JS
	ChatJS             template.JS
	FrameAncestors     string
}

// previewHostView 定义管理端挂件预览宿主页内容。
type previewHostView struct {
	Lang       string
	Title      string
	StageLabel string
}

var pageTemplate = template.Must(template.New("chat").Parse(pageHTML))
var previewHostTemplate = template.Must(template.New("preview").Parse(previewHTML))

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
	case request.URL.Path == "/preview/frame":
		// 返回管理界面使用的 Messenger 预览页。
		locale := cervii18n.PreferredCustomerLocale(request.Header.Get("Accept-Language"))
		page := baseView("preview", defaultTheme(), locale)
		page.Preview = true
		page.ShowWidgetControls = true
		// 预览框同时允许管理端顶层和同源预览宿主页。
		page.FrameAncestors = "* wails:"
		page.Title = cervii18n.LocalizeCustomerTemplate(locale, cervii18n.MessengerDefaultTitle, nil)
		page.TitleInitials = nameInitials(page.Title)
		page.Greeting = page.Copy["conversationPrompt"]
		if err := writePage(writer, page, http.StatusOK); err != nil {
			slog.Warn("写入网站渠道 Messenger 预览框失败", "error", err)
		}
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
	channelID := strings.TrimPrefix(request.URL.Path, "/")
	if channelID == "assets/markdown.js" || channelID == "assets/markdown.css" {
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFileFS(writer, request, markdownAssets, "dist/"+strings.TrimPrefix(channelID, "assets/"))
		return
	}
	if channelID == "preview" {
		if err := writePreviewHost(writer, request); err != nil {
			slog.Warn("写入网站渠道挂件预览失败", "error", err)
		}
		return
	}
	writeChatPage(writer, request, s.lookup, channelID, "link")
}

// writePreviewHost 写入管理端挂件预览宿主页。
func writePreviewHost(writer http.ResponseWriter, request *http.Request) error {
	acceptLanguage := request.Header.Get("Accept-Language")
	title, lang := cervii18n.Localize(acceptLanguage, cervii18n.MessengerPreviewTitle)
	stageLabel, _ := cervii18n.Localize(acceptLanguage, cervii18n.MessengerPreviewStageLabel)
	view := previewHostView{Lang: lang, Title: title, StageLabel: stageLabel}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Vary", "Accept-Language")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Security-Policy", "frame-ancestors * wails:")
	return previewHostTemplate.Execute(writer, view)
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
	cacheControl := "public, max-age=300"
	if request.URL.Query().Get("preview") == "1" {
		cacheControl = "no-store"
	}
	if channelID != "" {
		channel, err := s.lookup(request.Context(), channelID)
		if errors.Is(err, channelaction.ErrNotFound) {
			slog.Info("网站嵌入脚本渠道不存在", "channel_id", channelID)
			writeWidgetJavaScript(writer, http.StatusNotFound, "no-store", channelID, []byte("/* Cervi: website channel not found. */"))
			return
		}
		if err != nil {
			slog.Warn("读取网站嵌入脚本渠道失败", "channel_id", channelID, "error", err)
			writeWidgetJavaScript(writer, http.StatusInternalServerError, "no-store", channelID, []byte("/* Cervi: website channel unavailable. */"))
			return
		}
		host := embedRequestHost(request)
		if !embedhost.Allows(channel.AllowedEmbedHosts, host) {
			slog.Info("网站渠道拒绝未允许的嵌入脚本来源", "channel_id", channelID, "host", host)
			writeWidgetJavaScript(writer, http.StatusForbidden, "no-store", channelID, []byte("/* Cervi: this website is not allowed to use the channel. */"))
			return
		}
		theme = parseTheme(channel.ThemeColor)
	}
	// 生成挂件主题变量。
	hostCSS := fmt.Sprintf(
		":host{--cv-theme:%s;--cv-on-theme:%s;--cv-focus:%s;--cv-launcher-shadow:%s}",
		theme.Color,
		theme.OnColor,
		theme.Focus,
		theme.LauncherShadow,
	)
	// 按访客语言偏好生成挂件文案。
	copyJSON, _ := json.Marshal(cervii18n.LocalizeCustomerMap(cervii18n.PreferredCustomerLocale(request.Header.Get("Accept-Language")), map[string]cervii18n.Key{
		"dialog": cervii18n.MessengerWidgetDialog,
		"open":   cervii18n.MessengerWidgetOpen,
		"close":  cervii18n.MessengerClose,
	}))
	// 生成包含主题变量与挂件文案的挂件脚本。
	script := bytes.Replace(widgetScript, []byte(themePlaceholder), []byte(hostCSS), 1)
	script = bytes.Replace(script, []byte(copyPlaceholder), copyJSON, 1)
	writeWidgetJavaScript(writer, http.StatusOK, cacheControl, channelID, script)
}

// writeWidgetJavaScript 写入网站嵌入脚本响应。
func writeWidgetJavaScript(writer http.ResponseWriter, status int, cacheControl string, channelID string, script []byte) {
	writer.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	writer.Header().Set("Cache-Control", cacheControl)
	writer.Header().Set("Vary", "Origin, Referer, Accept-Language")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	if _, err := writer.Write(script); err != nil {
		slog.Warn("写入网站嵌入脚本响应失败", "channel_id", channelID, "status", status, "error", err)
	}
}

// writeChatPage 渲染公开聊天页。
func writeChatPage(writer http.ResponseWriter, request *http.Request, lookup Lookup, channelID string, entry string) {
	channel, err := lookup(request.Context(), channelID)
	if errors.Is(err, channelaction.ErrNotFound) {
		// 返回聊天入口不存在时的页面。
		locale := cervii18n.PreferredCustomerLocale(request.Header.Get("Accept-Language"))
		page := baseView(entry, defaultTheme(), locale)
		page.NotFound = true
		messages := cervii18n.LocalizeCustomerMap(locale, map[string]cervii18n.Key{
			"title":   cervii18n.MessengerUnavailableTitle,
			"message": cervii18n.MessengerUnavailableMessage,
		})
		page.Title = messages["title"]
		page.EmptyMessage = messages["message"]
		if err := writePage(writer, page, http.StatusNotFound); err != nil {
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
	if entry == "embed" {
		host := embedRequestHost(request)
		if !embedhost.Allows(channel.AllowedEmbedHosts, host) {
			// 拒绝未允许的网站加载聊天框。
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.Header().Set("Cache-Control", "no-store")
			writer.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
			writer.Header().Set("Vary", "Origin, Referer")
			writer.Header().Set("X-Content-Type-Options", "nosniff")
			writer.WriteHeader(http.StatusForbidden)
			slog.Info("网站渠道拒绝未允许的嵌入聊天框来源", "channel_id", channel.ID, "host", host)
			return
		}
	}
	locale := cervii18n.PreferredCustomerLocale(request.Header.Get("Accept-Language"))
	if err := writePage(writer, chatView(channel, entry, locale), http.StatusOK); err != nil {
		slog.Warn("写入网站渠道聊天页失败", "channel_id", channel.ID, "entry", entry, "error", err)
		return
	}
	slog.Info("打开网站渠道聊天页", "channel_id", channel.ID, "entry", entry)
}

// writePage 写入公开聊天 HTML。
func writePage(writer http.ResponseWriter, page pageView, status int) error {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Vary", "Accept-Language")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Security-Policy", "frame-ancestors "+page.FrameAncestors)
	writer.WriteHeader(status)
	return pageTemplate.Execute(writer, page)
}

// chatView 按渠道设置生成聊天页。
func chatView(channel *channelaction.PublicWebsiteChannel, entry string, locale domain.CustomerLocale) pageView {
	page := baseView(entry, parseTheme(channel.ThemeColor), locale)
	page.ChannelID = channel.ID
	page.Title = channel.Title
	page.TitleInitials = nameInitials(channel.Title)
	page.HomeLinks = channel.HomeLinks
	page.Greeting = strings.TrimSpace(channel.Greeting)
	if page.Greeting == "" {
		page.Greeting = page.Copy["conversationPrompt"]
	}
	if entry == "embed" {
		page.FrameAncestors = embedhost.FrameAncestors(channel.AllowedEmbedHosts)
	}
	return page
}

// baseView 填充聊天页共用内容。
func baseView(entry string, theme theme, locale domain.CustomerLocale) pageView {
	// 按映射表本地化 Messenger 固定文案。
	messengerText := cervii18n.LocalizeCustomerMap(locale, messengerCopyMessageKeys)
	page := pageView{
		Shell:              entry,
		ShowWidgetControls: entry == "embed",
		Copy:               messengerText,
		ThemeCSS:           template.CSS(theme.rootCSS()),
		MessengerCSS:       template.CSS(messengerCSS),
		ComposerEmojis:     template.JS(composerEmojisJSON),
		ChatJS:             template.JS(chatJS),
		FrameAncestors:     "*",
		Lang:               string(locale),
	}
	return page
}

// nameInitials 取名称开头的两个字符作为字标，名称为空时返回问号。
func nameInitials(name string) string {
	characters := []rune(strings.ToUpper(strings.TrimSpace(name)))
	if len(characters) == 0 {
		return "?"
	}
	return string(characters[:min(len(characters), 2)])
}

// messengerCopyMessageKeys 是访客 Messenger 固定文案的唯一定义：
// 键即模板中 .Copy 的字段名，新增文案只需在此补一条并在模板引用。
var messengerCopyMessageKeys = map[string]cervii18n.Key{
	"home":                   cervii18n.MessengerHome,
	"messages":               cervii18n.MessengerMessages,
	"help":                   cervii18n.MessengerHelp,
	"message":                cervii18n.MessengerMessage,
	"close":                  cervii18n.MessengerClose,
	"attach":                 cervii18n.MessengerAttach,
	"emoji":                  cervii18n.MessengerEmoji,
	"demoReply":              cervii18n.MessengerDemoReply,
	"welcome":                cervii18n.MessengerWelcome,
	"howCanWeHelp":           cervii18n.MessengerHowCanWeHelp,
	"startConversation":      cervii18n.MessengerStartConversation,
	"replyImmediate":         cervii18n.MessengerReplyImmediate,
	"replySoon":              cervii18n.MessengerReplySoon,
	"replyScheduled":         cervii18n.MessengerReplyScheduled,
	"noMessages":             cervii18n.MessengerNoMessages,
	"noMessagesDescription":  cervii18n.MessengerNoMessagesDescription,
	"searchHelp":             cervii18n.MessengerSearchHelp,
	"noHelpResults":          cervii18n.MessengerNoHelpResults,
	"back":                   cervii18n.MessengerBack,
	"stillNeedHelp":          cervii18n.MessengerStillNeedHelp,
	"articleCount":           cervii18n.MessengerArticleCount,
	"articleCountOne":        cervii18n.MessengerArticleCountOne,
	"collectionCount":        cervii18n.MessengerCollectionCount,
	"collectionCountOne":     cervii18n.MessengerCollectionCountOne,
	"helpSearching":          cervii18n.MessengerHelpSearching,
	"helpSearchFailed":       cervii18n.MessengerHelpSearchFailed,
	"helpArticleUnavailable": cervii18n.MessengerHelpArticleUnavailable,
	"conversationPrompt":     cervii18n.MessengerConversationPrompt,
	"more":                   cervii18n.MessengerMore,
	"expandWindow":           cervii18n.MessengerExpandWindow,
	"collapseWindow":         cervii18n.MessengerCollapseWindow,
	"recordVoice":            cervii18n.MessengerRecordVoice,
	"playVoice":              cervii18n.MessengerPlayVoice,
	"pauseVoice":             cervii18n.MessengerPauseVoice,
	"send":                   cervii18n.MessengerSend,
	"cancelRecording":        cervii18n.MessengerCancelRecording,
	"stopRecording":          cervii18n.MessengerStopRecording,
	"messengerNavigation":    cervii18n.MessengerNavigation,
	"loading":                cervii18n.MessengerLoading,
	"retry":                  cervii18n.MessengerRetry,
	"referenceDeleted":       cervii18n.MessengerReferenceDeleted,
	"referenceVisitor":       cervii18n.MessengerReferenceVisitor,
	"referenceAgent":         cervii18n.MessengerReferenceAgent,
	"referenceReply":         cervii18n.MessengerReferenceReply,
	"referenceReplying":      cervii18n.MessengerReferenceReplying,
	"referenceCancel":        cervii18n.MessengerReferenceCancel,
	"referenceUnavailable":   cervii18n.MessengerReferenceUnavailable,
	"referenceLatest":        cervii18n.MessengerReferenceLatest,
	"requestFailed":          cervii18n.MessengerRequestFailed,
	"identityExpired":        cervii18n.MessengerIdentityExpired,
	"attachmentUploading":    cervii18n.MessengerAttachmentUploading,
	"attachmentFailed":       cervii18n.MessengerAttachmentFailed,
	"attachmentCancel":       cervii18n.MessengerAttachmentCancel,
	"attachmentReceiving":    cervii18n.MessengerAttachmentReceiving,
	"attachmentUnavailable":  cervii18n.MessengerAttachmentUnavailable,
	"sessionOpen":            cervii18n.MessengerSessionOpen,
	"sessionClosed":          cervii18n.MessengerSessionClosed,
	"dayToday":               cervii18n.MessengerDayToday,
	"dayYesterday":           cervii18n.MessengerDayYesterday,
	"sessionEnded":           cervii18n.MessengerSessionEnded,
	"memberJoined":           cervii18n.MessengerMemberJoined,
	"emailCollected":         cervii18n.MessengerEmailCollected,
	"ratingQuestion":         cervii18n.MessengerRatingQuestion,
	"ratingResolved":         cervii18n.MessengerRatingResolved,
	"ratingUnresolved":       cervii18n.MessengerRatingUnresolved,
	"ratingComment":          cervii18n.MessengerRatingComment,
	"ratingSubmit":           cervii18n.MessengerRatingSubmit,
	"ratingThanks":           cervii18n.MessengerRatingThanks,
}

// embedRequestHost 从公开嵌入请求中读取宿主网站主机。
func embedRequestHost(request *http.Request) string {
	for _, value := range []string{request.Header.Get("Origin"), request.Referer()} {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}
	return ""
}
