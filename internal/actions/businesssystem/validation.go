//go:build server

package businesssystem

import (
	"strings"
	"unicode/utf8"

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

const (
	// maxNameLength 是业务系统名称的最大字符数。
	maxNameLength = 100
	// maxDescriptionLength 是业务系统描述的最大字符数。
	maxDescriptionLength = 200
	// maxURLBytes 是业务系统地址的最大字节数。
	maxURLBytes = 2048
)

// normalizeInput 归一化并校验业务系统输入。
func normalizeInput(input Input) (Input, map[string]ValidationCode) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.URL = strings.TrimSpace(input.URL)
	fields := make(map[string]ValidationCode)
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if utf8.RuneCountInString(input.Name) > maxNameLength {
		fields["name"] = ValidationNameTooLong
	}
	if utf8.RuneCountInString(input.Description) > maxDescriptionLength {
		fields["description"] = ValidationDescriptionTooLong
	}
	if input.URL == "" {
		fields["url"] = ValidationURLRequired
	} else if len(input.URL) > maxURLBytes {
		fields["url"] = ValidationURLTooLong
	} else if !common.ValidHTTPURL(input.URL) {
		fields["url"] = ValidationURLInvalid
	}
	return input, fields
}
