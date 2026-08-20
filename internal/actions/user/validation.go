//go:build server

package user

import (
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
)

// ValidationCode 标识个人资料字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationDisplayNameRequired ValidationCode = "DISPLAY_NAME_REQUIRED"
	ValidationEmailInvalid        ValidationCode = "EMAIL_INVALID"
	ValidationEmailDuplicate      ValidationCode = "EMAIL_DUPLICATE"
)

// ValidationError 表示个人资料字段校验失败。
type ValidationError = common.FieldError

// ProfileInput 定义当前用户可编辑的个人资料字段。
type ProfileInput struct {
	DisplayName string
	Email       string
}

// normalizeProfileInput 规范化并校验个人资料输入。
func normalizeProfileInput(input ProfileInput) (ProfileInput, map[string]ValidationCode) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = commonemail.Normalize(input.Email)

	fields := make(map[string]ValidationCode)
	if input.DisplayName == "" {
		fields["displayName"] = ValidationDisplayNameRequired
	}
	if !commonemail.Valid(input.Email) {
		fields["email"] = ValidationEmailInvalid
	}
	return input, fields
}
