//go:build server

package businesssystem

import (
	"net/url"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
)

// ValidationCode 标识业务系统字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationNameRequired       ValidationCode = "BUSINESS_SYSTEM_NAME_REQUIRED"
	ValidationNameTooLong        ValidationCode = "BUSINESS_SYSTEM_NAME_TOO_LONG"
	ValidationNameDuplicate      ValidationCode = "BUSINESS_SYSTEM_NAME_DUPLICATE"
	ValidationDescriptionTooLong ValidationCode = "BUSINESS_SYSTEM_DESCRIPTION_TOO_LONG"
	ValidationURLRequired        ValidationCode = "BUSINESS_SYSTEM_URL_REQUIRED"
	ValidationURLInvalid         ValidationCode = "BUSINESS_SYSTEM_URL_INVALID"
	ValidationURLTooLong         ValidationCode = "BUSINESS_SYSTEM_URL_TOO_LONG"
)

// ValidationError 表示业务系统字段校验失败。
type ValidationError = common.FieldError

// normalizeInput 归一化并校验业务系统输入。
func normalizeInput(input Input) (Input, map[string]ValidationCode) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.URL = strings.TrimSpace(input.URL)
	fields := make(map[string]ValidationCode)
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if len([]rune(input.Name)) > 100 {
		fields["name"] = ValidationNameTooLong
	}
	if len([]rune(input.Description)) > 200 {
		fields["description"] = ValidationDescriptionTooLong
	}
	if input.URL == "" {
		fields["url"] = ValidationURLRequired
	} else if len(input.URL) > 2048 {
		fields["url"] = ValidationURLTooLong
	} else if !validURL(input.URL) {
		fields["url"] = ValidationURLInvalid
	}
	return input, fields
}

// validURL 校验地址为不含认证信息的完整 HTTP 或 HTTPS 地址。
func validURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.Host != "" && parsed.User == nil &&
		(strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https"))
}
