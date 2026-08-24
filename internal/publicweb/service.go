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
	"net/url"
	"strings"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/common/embedhost"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const themePlaceholder = "/*CV_THEME*/"

// Lookup 按渠道标识读取公开网站渠道。
type Lookup func(context.Context, string) (*channelaction.PublicWebsiteChannel, error)

type pageView struct {
	Lang               string
	Title              string
	Subtitle           string
	Agent              serviceAgentView
	Greeting           string
	EmptyMessage       string
	Shell              string
	NotFound           bool
	ShowWidgetControls bool
	Preview            bool
	Copy               messengerCopy
	ThemeCSS           template.CSS
	ChromeCSS          template.CSS
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

// messengerCopy 定义访客 Messenger 的固定本地化文案。
type messengerCopy struct {
	Home                      string
	Messages                  string
	Help                      string
	Message                   string
	Close                     string
	Attach                    string
	Emoji                     string
	DefaultAgentName          string
	DefaultAgentLastActive    string
	DemoReply                 string
	Welcome                   string
	HowCanWeHelp              string
	StartConversation         string
	DefaultResponse           string
	ExploreHelp               string
	ExploreHelpDescription    string
	ViewAll                   string
	GettingStarted            string
	GettingStartedDescription string
	FeaturesAndSettings       string
	FeaturesDescription       string
	CommonQuestions           string
	QuestionsDescription      string
	NoMessages                string
	NoMessagesDescription     string
	SearchHelp                string
	Collections               string
	CollectionCount           string
	ThreeArticles             string
	FiveArticles              string
	SixArticles               string
	NoHelpResults             string
	Back                      string
	ArticleOneTitle           string
	ArticleOneBody            string
	ArticleTwoTitle           string
	ArticleTwoBody            string
	ArticleThreeTitle         string
	ArticleThreeBody          string
	StillNeedHelp             string
	ConversationPrompt        string
	More                      string
	ExpandWindow              string
	CollapseWindow            string
	WaitingForTeam            string
	RecordVoice               string
	PlayVoice                 string
	PauseVoice                string
	Send                      string
	CancelRecording           string
	StopRecording             string
	MessengerNavigation       string
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
		if err := writePage(writer, previewView(request), http.StatusOK); err != nil {
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
	view := previewHostView{
		Lang:       "en-US",
		Title:      "Widget preview",
		StageLabel: "Website widget preview",
	}
	if locale == domain.LocaleChineseSimplified {
		view.Lang = "zh-CN"
		view.Title = "挂件预览"
		view.StageLabel = "网站挂件预览"
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
	writeWidgetJavaScript(writer, http.StatusOK, cacheControl, channelID, scriptWithTheme(theme))
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

// scriptWithTheme 生成包含主题变量的挂件脚本。
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
	if entry == "embed" {
		host := embedRequestHost(request)
		if !embedhost.Allows(channel.AllowedEmbedHosts, host) {
			writeEmbedFrameForbidden(writer)
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
	page.Title = channel.Title
	page.Subtitle = strings.TrimSpace(channel.Subtitle)
	if page.Subtitle == "" {
		page.Subtitle = page.Copy.DefaultResponse
	}
	page.Greeting = strings.TrimSpace(channel.Greeting)
	if page.Greeting == "" {
		page.Greeting = page.Copy.ConversationPrompt
	}
	if entry == "embed" {
		page.FrameAncestors = embedhost.FrameAncestors(channel.AllowedEmbedHosts)
	}
	return page
}

// previewView 返回管理界面使用的 Messenger 预览页。
func previewView(request *http.Request) pageView {
	locale := preferredMessengerLocale(request.Header.Get("Accept-Language"))
	page := baseView("preview", defaultTheme(), locale)
	page.Preview = true
	page.ShowWidgetControls = true
	// 预览框同时允许管理端顶层和同源预览宿主页。
	page.FrameAncestors = "* wails:"
	page.Title = "Support"
	if locale == domain.LocaleChineseSimplified {
		page.Title = "在线咨询"
	}
	page.Subtitle = page.Copy.DefaultResponse
	page.Greeting = page.Copy.ConversationPrompt
	return page
}

// notFoundView 返回聊天入口不存在时的页面。
func notFoundView(request *http.Request, entry string) pageView {
	locale := preferredMessengerLocale(request.Header.Get("Accept-Language"))
	page := baseView(entry, defaultTheme(), locale)
	page.NotFound = true
	if locale == domain.LocaleChineseSimplified {
		page.Title = "无法打开聊天"
		page.EmptyMessage = "这个聊天入口不可用。"
	} else {
		page.Title = "Chat unavailable"
		page.EmptyMessage = "This chat link is not available."
	}
	return page
}

// baseView 填充聊天页共用内容。
func baseView(entry string, theme theme, locale domain.Locale) pageView {
	copy := localizedMessengerCopy(locale)
	page := pageView{
		Shell:              entry,
		ShowWidgetControls: entry == "embed",
		Copy:               copy,
		Agent: serviceAgentView{
			Name:       copy.DefaultAgentName,
			LastActive: copy.DefaultAgentLastActive,
			Initials:   agentInitials(copy.DefaultAgentName),
		},
		ThemeCSS:       template.CSS(theme.rootCSS()),
		ChromeCSS:      template.CSS(chromeCSS),
		ComposerEmojis: template.JS(composerEmojisJSON),
		ChatJS:         template.JS(chatJS),
		FrameAncestors: "*",
	}
	if locale == domain.LocaleChineseSimplified {
		page.Lang = "zh-CN"
	} else {
		page.Lang = "en-US"
	}
	return page
}

// localizedMessengerCopy 返回 Messenger 固定文案。
func localizedMessengerCopy(locale domain.Locale) messengerCopy {
	if locale != domain.LocaleChineseSimplified {
		return messengerCopy{
			Home:                      "Home",
			Messages:                  "Messages",
			Help:                      "Help",
			Message:                   "Message",
			Close:                     "Close chat",
			Attach:                    "Attach file",
			Emoji:                     "Choose emoji",
			DefaultAgentName:          "Support team",
			DefaultAgentLastActive:    "Last active just now",
			DemoReply:                 "Thanks, we have your message. A teammate can continue from here.",
			Welcome:                   "Hi there 👋",
			HowCanWeHelp:              "How can we help?",
			StartConversation:         "Start a chat",
			DefaultResponse:           "We usually reply as soon as we can",
			ExploreHelp:               "Explore help",
			ExploreHelpDescription:    "Find a quick answer before you wait",
			ViewAll:                   "View all",
			GettingStarted:            "Getting started",
			GettingStartedDescription: "The essentials for getting up and running",
			FeaturesAndSettings:       "Features and settings",
			FeaturesDescription:       "Learn how the product works and make it yours",
			CommonQuestions:           "Common questions",
			QuestionsDescription:      "Answers to the questions people ask most",
			NoMessages:                "No messages",
			NoMessagesDescription:     "Messages from the team will be shown here",
			SearchHelp:                "Search help",
			Collections:               "Help center",
			CollectionCount:           "3 collections",
			ThreeArticles:             "3 articles",
			FiveArticles:              "5 articles",
			SixArticles:               "6 articles",
			NoHelpResults:             "No matching help content",
			Back:                      "Back",
			ArticleOneTitle:           "Start a conversation",
			ArticleOneBody:            "Open the Messenger and send a message. You can come back here at any time to continue.",
			ArticleTwoTitle:           "Find your recent messages",
			ArticleTwoBody:            "The Messages space keeps your conversations together and makes it easy to pick up where you left off.",
			ArticleThreeTitle:         "Get more help",
			ArticleThreeBody:          "Browse the Help space or start a conversation when you need a hand from the team.",
			StillNeedHelp:             "Still need help?",
			ConversationPrompt:        "Ask us anything, or share your feedback.",
			More:                      "More",
			ExpandWindow:              "Expand window",
			CollapseWindow:            "Restore window",
			WaitingForTeam:            "Waiting for a teammate",
			RecordVoice:               "Record voice message",
			PlayVoice:                 "Play voice message",
			PauseVoice:                "Pause voice message",
			Send:                      "Send",
			CancelRecording:           "Cancel recording",
			StopRecording:             "Stop recording",
			MessengerNavigation:       "Messenger navigation",
		}
	}
	return messengerCopy{
		Home:                      "首页",
		Messages:                  "消息",
		Help:                      "帮助",
		Message:                   "消息",
		Close:                     "关闭聊天",
		Attach:                    "添加附件",
		Emoji:                     "选择表情",
		DefaultAgentName:          "客服团队",
		DefaultAgentLastActive:    "上次活跃：刚刚",
		DemoReply:                 "谢谢，我们已经收到你的消息，接下来可以由团队成员继续回复。",
		Welcome:                   "你好 👋",
		HowCanWeHelp:              "需要什么帮助？",
		StartConversation:         "开始聊天",
		DefaultResponse:           "我们通常会尽快回复",
		ExploreHelp:               "自助查找",
		ExploreHelpDescription:    "也许这里就有你需要的答案",
		ViewAll:                   "查看全部",
		GettingStarted:            "开始使用",
		GettingStartedDescription: "快速了解基本使用方式",
		FeaturesAndSettings:       "功能与设置",
		FeaturesDescription:       "了解主要功能并完成个性化设置",
		CommonQuestions:           "常见问题",
		QuestionsDescription:      "查看大家最常遇到的问题",
		NoMessages:                "暂无消息",
		NoMessagesDescription:     "团队发来的消息会显示在这里",
		SearchHelp:                "搜索帮助内容",
		Collections:               "帮助中心",
		CollectionCount:           "3 个主题",
		ThreeArticles:             "3 篇文章",
		FiveArticles:              "5 篇文章",
		SixArticles:               "6 篇文章",
		NoHelpResults:             "没有找到匹配的帮助内容",
		Back:                      "返回",
		ArticleOneTitle:           "发起一段对话",
		ArticleOneBody:            "打开 Messenger 并发送消息，你可以随时回到这里继续之前的交流。",
		ArticleTwoTitle:           "查看最近消息",
		ArticleTwoBody:            "消息页会集中保存你的对话，方便从上次停下的位置继续。",
		ArticleThreeTitle:         "获得更多帮助",
		ArticleThreeBody:          "你可以浏览帮助内容，也可以随时与团队聊聊。",
		StillNeedHelp:             "仍然需要帮助？",
		ConversationPrompt:        "有任何问题或建议，都可以告诉我们。",
		More:                      "更多",
		ExpandWindow:              "展开窗口",
		CollapseWindow:            "恢复窗口",
		WaitingForTeam:            "正在等待团队成员",
		RecordVoice:               "发送语音消息",
		PlayVoice:                 "播放语音消息",
		PauseVoice:                "暂停语音消息",
		Send:                      "发送",
		CancelRecording:           "取消录音",
		StopRecording:             "结束录音",
		MessengerNavigation:       "Messenger 导航",
	}
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

// writeEmbedFrameForbidden 拒绝未允许的网站加载聊天框。
func writeEmbedFrameForbidden(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
	writer.Header().Set("Vary", "Origin, Referer")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(http.StatusForbidden)
}

// agentInitials 返回客服名称开头的两个字标。
func agentInitials(value string) string {
	characters := []rune(strings.ToUpper(strings.TrimSpace(value)))
	if len(characters) == 0 {
		return "?"
	}
	if len(characters) > 2 {
		characters = characters[:2]
	}
	return string(characters)
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
