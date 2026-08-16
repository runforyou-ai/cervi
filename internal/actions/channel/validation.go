//go:build server

// Package channel 实现消息渠道领域的应用操作。
package channel

import (
	"strings"

	"github.com/google/uuid"
)

const (
	// TypeWebsite 标识网站消息渠道。
	TypeWebsite = "website"
	// LocaleChineseSimplified 标识简体中文访客语言。
	LocaleChineseSimplified = "zh-CN"
	// LocaleEnglishUnitedStates 标识美式英语访客语言。
	LocaleEnglishUnitedStates = "en-US"
)

// ValidationCode 标识网站渠道字段校验结果。
type ValidationCode string

const (
	ValidationNameRequired         ValidationCode = "NAME_REQUIRED"
	ValidationNameTooLong          ValidationCode = "NAME_TOO_LONG"
	ValidationDescriptionTooLong   ValidationCode = "DESCRIPTION_TOO_LONG"
	ValidationDefaultLocaleInvalid ValidationCode = "DEFAULT_LOCALE_INVALID"
)

// ValidationError 返回网站渠道字段校验结果。
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
	DefaultLocale string
}

// normalizeWebsiteChannelInput 规范化并校验网站渠道输入。
func normalizeWebsiteChannelInput(input WebsiteChannelInput) (WebsiteChannelInput, map[string]ValidationCode) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.DefaultLocale = strings.TrimSpace(input.DefaultLocale)

	fields := make(map[string]ValidationCode)
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if len([]rune(input.Name)) > 100 {
		fields["name"] = ValidationNameTooLong
	}
	if len([]rune(input.Description)) > 2000 {
		fields["description"] = ValidationDescriptionTooLong
	}
	if input.DefaultLocale != LocaleChineseSimplified && input.DefaultLocale != LocaleEnglishUnitedStates {
		fields["defaultLocale"] = ValidationDefaultLocaleInvalid
	}
	return input, fields
}

// validUUID 判断记录标识是否为 UUID。
func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && strings.EqualFold(parsed.String(), value)
}
