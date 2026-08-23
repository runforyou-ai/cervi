//go:build server

// Package channel 实现消息渠道领域的应用操作。
package channel

import (
	"regexp"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/embedhost"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const (
	// DefaultWebsiteChannelThemeColor 是网站聊天界面的默认主题色。
	DefaultWebsiteChannelThemeColor = "#2563EB"
)

var themeColorPattern = regexp.MustCompile(`^#[0-9A-F]{6}$`)

// ValidationCode 标识网站渠道字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationTypeInvalid          ValidationCode = "TYPE_INVALID"
	ValidationNameRequired         ValidationCode = "NAME_REQUIRED"
	ValidationNameTooLong          ValidationCode = "NAME_TOO_LONG"
	ValidationDescriptionTooLong   ValidationCode = "DESCRIPTION_TOO_LONG"
	ValidationDefaultLocaleInvalid ValidationCode = "DEFAULT_LOCALE_INVALID"
	ValidationRoutingTargetInvalid ValidationCode = "ROUTING_TARGET_INVALID"
	ValidationChatTitleRequired    ValidationCode = "CHAT_TITLE_REQUIRED"
	ValidationChatTitleTooLong     ValidationCode = "CHAT_TITLE_TOO_LONG"
	ValidationChatSubtitleTooLong  ValidationCode = "CHAT_SUBTITLE_TOO_LONG"
	ValidationGreetingTooLong      ValidationCode = "GREETING_MESSAGE_TOO_LONG"
	ValidationThemeColorInvalid    ValidationCode = "THEME_COLOR_INVALID"
	ValidationAllowedHostsTooMany  ValidationCode = "ALLOWED_HOSTS_TOO_MANY"
	ValidationAllowedHostInvalid   ValidationCode = "ALLOWED_HOST_INVALID"
)

// ValidationError 表示网站渠道字段校验失败。
type ValidationError = common.FieldError

// WebsiteChannelInput 定义网站渠道基础字段。
type WebsiteChannelInput struct {
	Type                  domain.ChannelType
	Name                  string
	Description           string
	DefaultLocale         domain.Locale
	NewConversationTarget RoutingTarget
	FallbackTarget        RoutingTarget
}

// RoutingTarget 定义渠道会话流转目标。
type RoutingTarget struct {
	Type domain.ChannelRoutingTargetType
	ID   string
}

// WebsiteChannelChatInterfaceInput 定义网站渠道聊天界面可编辑字段。
type WebsiteChannelChatInterfaceInput struct {
	Title           string
	Subtitle        string
	GreetingMessage string
	ThemeColor      string
}

// WebsiteChannelAccessInput 定义网站渠道允许使用的网站输入。
type WebsiteChannelAccessInput struct {
	AllowedHosts []string
}

// normalizeWebsiteChannelInput 规范化并校验网站渠道输入。
func normalizeWebsiteChannelInput(input WebsiteChannelInput) (WebsiteChannelInput, map[string]ValidationCode) {
	input.Type = domain.ChannelType(strings.TrimSpace(string(input.Type)))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.DefaultLocale = domain.Locale(strings.TrimSpace(string(input.DefaultLocale)))
	input.NewConversationTarget = normalizeRoutingTarget(input.NewConversationTarget)
	input.FallbackTarget = normalizeRoutingTarget(input.FallbackTarget)

	fields := make(map[string]ValidationCode)
	if input.Type != domain.ChannelTypeWebsite {
		fields["type"] = ValidationTypeInvalid
	}
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if len([]rune(input.Name)) > 100 {
		fields["name"] = ValidationNameTooLong
	}
	if len([]rune(input.Description)) > 2000 {
		fields["description"] = ValidationDescriptionTooLong
	}
	if input.DefaultLocale != domain.LocaleChineseSimplified && input.DefaultLocale != domain.LocaleEnglishUnitedStates {
		fields["defaultLocale"] = ValidationDefaultLocaleInvalid
	}
	if !routingTargetShapeValid(input.NewConversationTarget) {
		fields["newConversationTarget"] = ValidationRoutingTargetInvalid
	}
	if !routingTargetShapeValid(input.FallbackTarget) {
		fields["fallbackTarget"] = ValidationRoutingTargetInvalid
	}
	if input.NewConversationTarget.Type != domain.ChannelRoutingTargetTypePublicQueue &&
		input.NewConversationTarget.Type == input.FallbackTarget.Type &&
		input.NewConversationTarget.ID == input.FallbackTarget.ID {
		fields["fallbackTarget"] = ValidationRoutingTargetInvalid
	}
	return input, fields
}

// normalizeRoutingTarget 规范化会话流转目标。
func normalizeRoutingTarget(target RoutingTarget) RoutingTarget {
	target.Type = domain.ChannelRoutingTargetType(strings.TrimSpace(string(target.Type)))
	target.ID = strings.TrimSpace(target.ID)
	return target
}

// routingTargetShapeValid 校验会话流转目标的字段组合。
func routingTargetShapeValid(target RoutingTarget) bool {
	switch target.Type {
	case domain.ChannelRoutingTargetTypePublicQueue:
		return target.ID == ""
	case domain.ChannelRoutingTargetTypeTeam, domain.ChannelRoutingTargetTypeMember:
		return common.ValidUUID(target.ID)
	default:
		return false
	}
}

// normalizeWebsiteChannelChatInterfaceInput 规范化并校验聊天界面输入。
func normalizeWebsiteChannelChatInterfaceInput(input WebsiteChannelChatInterfaceInput) (WebsiteChannelChatInterfaceInput, map[string]ValidationCode) {
	input.Title = strings.TrimSpace(input.Title)
	input.Subtitle = strings.TrimSpace(input.Subtitle)
	input.GreetingMessage = strings.TrimSpace(input.GreetingMessage)
	input.ThemeColor = strings.ToUpper(strings.TrimSpace(input.ThemeColor))

	fields := make(map[string]ValidationCode)
	if input.Title == "" {
		fields["title"] = ValidationChatTitleRequired
	} else if len([]rune(input.Title)) > 100 {
		fields["title"] = ValidationChatTitleTooLong
	}
	if len([]rune(input.Subtitle)) > 120 {
		fields["subtitle"] = ValidationChatSubtitleTooLong
	}
	if len([]rune(input.GreetingMessage)) > 500 {
		fields["greetingMessage"] = ValidationGreetingTooLong
	}
	if !themeColorPattern.MatchString(input.ThemeColor) {
		fields["themeColor"] = ValidationThemeColorInvalid
	}
	return input, fields
}

// normalizeWebsiteChannelAccessInput 规范化并校验允许使用的网站。
func normalizeWebsiteChannelAccessInput(input WebsiteChannelAccessInput) (WebsiteChannelAccessInput, map[string]ValidationCode) {
	fields := make(map[string]ValidationCode)
	if len(input.AllowedHosts) > embedhost.MaxHosts {
		fields["allowedHosts"] = ValidationAllowedHostsTooMany
		return input, fields
	}
	normalized, ok := embedhost.NormalizeAll(input.AllowedHosts)
	if !ok {
		fields["allowedHosts"] = ValidationAllowedHostInvalid
		return input, fields
	}
	input.AllowedHosts = normalized
	return input, fields
}
