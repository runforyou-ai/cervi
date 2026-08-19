//go:build server

// Package channel 实现消息渠道领域的应用操作。
package channel

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const (
	// DefaultWebsiteChannelThemeColor 是网站聊天界面的默认主题色。
	DefaultWebsiteChannelThemeColor = "#2563EB"
)

var themeColorPattern = regexp.MustCompile(`^#[0-9A-F]{6}$`)

// ValidationCode 标识网站渠道字段校验结果。
type ValidationCode string

const (
	ValidationNameRequired         ValidationCode = "NAME_REQUIRED"
	ValidationNameTooLong          ValidationCode = "NAME_TOO_LONG"
	ValidationDescriptionTooLong   ValidationCode = "DESCRIPTION_TOO_LONG"
	ValidationDefaultLocaleInvalid ValidationCode = "DEFAULT_LOCALE_INVALID"
	ValidationChatTitleRequired    ValidationCode = "CHAT_TITLE_REQUIRED"
	ValidationChatTitleTooLong     ValidationCode = "CHAT_TITLE_TOO_LONG"
	ValidationChatSubtitleTooLong  ValidationCode = "CHAT_SUBTITLE_TOO_LONG"
	ValidationGreetingTooLong      ValidationCode = "GREETING_MESSAGE_TOO_LONG"
	ValidationThemeColorInvalid    ValidationCode = "THEME_COLOR_INVALID"
)

// ValidationError 表示网站渠道字段校验失败。
type ValidationError struct {
	Fields map[string]ValidationCode
}

// Error 返回网站渠道输入校验错误。
func (e *ValidationError) Error() string {
	return "website channel validation failed"
}

// WebsiteChannelInput 定义网站渠道可编辑字段。
type WebsiteChannelInput struct {
	Name          string
	Description   string
	DefaultLocale domain.Locale
}

// WebsiteChannelChatInterfaceInput 定义网站渠道聊天界面可编辑字段。
type WebsiteChannelChatInterfaceInput struct {
	Title           string
	Subtitle        string
	GreetingMessage string
	ThemeColor      string
}

// normalizeWebsiteChannelInput 规范化并校验网站渠道输入。
func normalizeWebsiteChannelInput(input WebsiteChannelInput) (WebsiteChannelInput, map[string]ValidationCode) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.DefaultLocale = domain.Locale(strings.TrimSpace(string(input.DefaultLocale)))

	fields := make(map[string]ValidationCode)
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
	return input, fields
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

// validUUID 判断记录标识是否为 UUID。
func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && strings.EqualFold(parsed.String(), value)
}
