//go:build server

// Package publicweb 提供网站渠道的公开嵌入脚本和访客聊天页。
package publicweb

import (
	"bytes"
	"context"
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

// Lookup 按渠道标识读取公开网站渠道。
type Lookup func(context.Context, string) (*channelaction.PublicWebsiteChannel, error)

type pageView struct {
	Lang               string
	ChannelID          string
	Title              string
	Subtitle           string
	Agent              serviceAgentView
	Greeting           string
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

// serviceAgentView 定义会话顶部展示的客服身份。
type serviceAgentView struct {
	Name       string
	LastActive string
	Initials   string
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
		locale := preferredMessengerLocale(request.Header.Get("Accept-Language"))
		page := baseView("preview", defaultTheme(), locale)
		page.Preview = true
		page.ShowWidgetControls = true
		// 预览框同时允许管理端顶层和同源预览宿主页。
		page.FrameAncestors = "* wails:"
		page.Title, _ = cervii18n.Localize(string(locale), cervii18n.MessengerDefaultTitle)
		page.Subtitle = page.Copy["defaultResponse"]
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
	locale := preferredMessengerLocale(request.Header.Get("Accept-Language"))
	messages := cervii18n.LocalizeMap(string(locale), map[string]cervii18n.Key{
		"title":      cervii18n.MessengerPreviewTitle,
		"stageLabel": cervii18n.MessengerPreviewStageLabel,
	})
	view := previewHostView{
		Lang:       string(locale),
		Title:      messages["title"],
		StageLabel: messages["stageLabel"],
	}
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
	// 生成包含主题变量的挂件脚本。
	script := bytes.Replace(widgetScript, []byte(themePlaceholder), []byte(hostCSS), 1)
	writeWidgetJavaScript(writer, http.StatusOK, cacheControl, channelID, script)
}

// writeWidgetJavaScript 写入网站嵌入脚本响应。
func writeWidgetJavaScript(writer http.ResponseWriter, status int, cacheControl string, channelID string, script []byte) {
	writer.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	writer.Header().Set("Cache-Control", cacheControl)
	writer.Header().Set("Vary", "Origin, Referer")
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
		locale := preferredMessengerLocale(request.Header.Get("Accept-Language"))
		page := baseView(entry, defaultTheme(), locale)
		page.NotFound = true
		messages := cervii18n.LocalizeMap(string(locale), map[string]cervii18n.Key{
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
	locale := preferredMessengerLocale(request.Header.Get("Accept-Language"))
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
func chatView(channel *channelaction.PublicWebsiteChannel, entry string, locale domain.Locale) pageView {
	page := baseView(entry, parseTheme(channel.ThemeColor), locale)
	page.ChannelID = channel.ID
	page.Title = channel.Title
	page.Subtitle = strings.TrimSpace(channel.Subtitle)
	if page.Subtitle == "" {
		page.Subtitle = page.Copy["defaultResponse"]
	}
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
func baseView(entry string, theme theme, locale domain.Locale) pageView {
	// 返回按映射表本地化后的 Messenger 固定文案。
	messengerText := cervii18n.LocalizeMap(string(locale), messengerCopyMessageKeys)
	// 返回客服名称开头的两个字标。
	characters := []rune(strings.ToUpper(strings.TrimSpace(messengerText["defaultAgentName"])))
	initials := "?"
	if len(characters) > 0 {
		if len(characters) > 2 {
			characters = characters[:2]
		}
		initials = string(characters)
	}
	page := pageView{
		Shell:              entry,
		ShowWidgetControls: entry == "embed",
		Copy:               messengerText,
		Agent: serviceAgentView{
			Name:       messengerText["defaultAgentName"],
			LastActive: messengerText["defaultAgentLastActive"],
			Initials:   initials,
		},
		ThemeCSS:       template.CSS(theme.rootCSS()),
		MessengerCSS:   template.CSS(messengerCSS),
		ComposerEmojis: template.JS(composerEmojisJSON),
		ChatJS:         template.JS(chatJS),
		FrameAncestors: "*",
		Lang:           string(locale),
	}
	return page
}

// messengerCopyMessageKeys 是访客 Messenger 固定文案的唯一定义：
// 键即模板中 .Copy 的字段名，新增文案只需在此补一条并在模板引用。
var messengerCopyMessageKeys = map[string]cervii18n.Key{
	"home":                      cervii18n.MessengerHome,
	"messages":                  cervii18n.MessengerMessages,
	"help":                      cervii18n.MessengerHelp,
	"message":                   cervii18n.MessengerMessage,
	"resizeMessageInput":        cervii18n.MessengerResizeMessageInput,
	"close":                     cervii18n.MessengerClose,
	"attach":                    cervii18n.MessengerAttach,
	"emoji":                     cervii18n.MessengerEmoji,
	"defaultAgentName":          cervii18n.MessengerDefaultAgentName,
	"defaultAgentLastActive":    cervii18n.MessengerDefaultAgentLastActive,
	"demoReply":                 cervii18n.MessengerDemoReply,
	"welcome":                   cervii18n.MessengerWelcome,
	"howCanWeHelp":              cervii18n.MessengerHowCanWeHelp,
	"startConversation":         cervii18n.MessengerStartConversation,
	"defaultResponse":           cervii18n.MessengerDefaultResponse,
	"exploreHelp":               cervii18n.MessengerExploreHelp,
	"exploreHelpDescription":    cervii18n.MessengerExploreHelpDescription,
	"viewAll":                   cervii18n.MessengerViewAll,
	"gettingStarted":            cervii18n.MessengerGettingStarted,
	"gettingStartedDescription": cervii18n.MessengerGettingStartedDescription,
	"featuresAndSettings":       cervii18n.MessengerFeaturesAndSettings,
	"featuresDescription":       cervii18n.MessengerFeaturesDescription,
	"commonQuestions":           cervii18n.MessengerCommonQuestions,
	"questionsDescription":      cervii18n.MessengerQuestionsDescription,
	"noMessages":                cervii18n.MessengerNoMessages,
	"noMessagesDescription":     cervii18n.MessengerNoMessagesDescription,
	"searchHelp":                cervii18n.MessengerSearchHelp,
	"collections":               cervii18n.MessengerCollections,
	"collectionCount":           cervii18n.MessengerCollectionCount,
	"threeArticles":             cervii18n.MessengerThreeArticles,
	"fiveArticles":              cervii18n.MessengerFiveArticles,
	"sixArticles":               cervii18n.MessengerSixArticles,
	"noHelpResults":             cervii18n.MessengerNoHelpResults,
	"back":                      cervii18n.MessengerBack,
	"articleOneTitle":           cervii18n.MessengerArticleOneTitle,
	"articleOneBody":            cervii18n.MessengerArticleOneBody,
	"articleTwoTitle":           cervii18n.MessengerArticleTwoTitle,
	"articleTwoBody":            cervii18n.MessengerArticleTwoBody,
	"articleThreeTitle":         cervii18n.MessengerArticleThreeTitle,
	"articleThreeBody":          cervii18n.MessengerArticleThreeBody,
	"stillNeedHelp":             cervii18n.MessengerStillNeedHelp,
	"conversationPrompt":        cervii18n.MessengerConversationPrompt,
	"more":                      cervii18n.MessengerMore,
	"expandWindow":              cervii18n.MessengerExpandWindow,
	"collapseWindow":            cervii18n.MessengerCollapseWindow,
	"recordVoice":               cervii18n.MessengerRecordVoice,
	"playVoice":                 cervii18n.MessengerPlayVoice,
	"pauseVoice":                cervii18n.MessengerPauseVoice,
	"send":                      cervii18n.MessengerSend,
	"cancelRecording":           cervii18n.MessengerCancelRecording,
	"stopRecording":             cervii18n.MessengerStopRecording,
	"messengerNavigation":       cervii18n.MessengerNavigation,
	"loading":                   cervii18n.MessengerLoading,
	"retry":                     cervii18n.MessengerRetry,
	"requestFailed":             cervii18n.MessengerRequestFailed,
	"sessionOpen":               cervii18n.MessengerSessionOpen,
	"sessionClosed":             cervii18n.MessengerSessionClosed,
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

// preferredMessengerLocale 按浏览器首选语言选择 Messenger 支持的语言。
func preferredMessengerLocale(acceptLanguage string) domain.Locale {
	first := strings.TrimSpace(strings.Split(acceptLanguage, ",")[0])
	first = strings.ToLower(strings.TrimSpace(strings.Split(first, ";")[0]))
	if first == "zh" || strings.HasPrefix(first, "zh-") {
		return domain.LocaleChineseSimplified
	}
	return domain.LocaleEnglishUnitedStates
}
