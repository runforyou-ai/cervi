//go:build server

package user

import (
	"errors"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	commontimezone "github.com/runforyou-ai/cervi/internal/common/timezone"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识用户设置字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationDisplayNameRequired      ValidationCode = "DISPLAY_NAME_REQUIRED"
	ValidationEmailInvalid             ValidationCode = "EMAIL_INVALID"
	ValidationEmailDuplicate           ValidationCode = "EMAIL_DUPLICATE"
	ValidationCurrentPasswordIncorrect ValidationCode = "CURRENT_PASSWORD_INCORRECT"
	ValidationPasswordTooShort         ValidationCode = "PASSWORD_TOO_SHORT"
	ValidationPasswordTooLong          ValidationCode = "PASSWORD_TOO_LONG"
	ValidationLocaleInvalid            ValidationCode = "LOCALE_INVALID"
	ValidationTimeZoneInvalid          ValidationCode = "TIME_ZONE_INVALID"
)

// ValidationError 表示用户设置字段校验失败。
type ValidationError = common.FieldError

// ProfileInput 定义当前用户可编辑的个人资料字段。
type ProfileInput struct {
	DisplayName string
	Email       string
}

// ChangePasswordInput 定义当前用户修改密码所需字段。
type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

// PreferencesInput 定义当前用户的语言和时区设置。
type PreferencesInput struct {
	Locale   domain.Locale
	TimeZone string
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

// validateChangePasswordInput 校验新密码长度。
func validateChangePasswordInput(input ChangePasswordInput) map[string]ValidationCode {
	fields := make(map[string]ValidationCode)
	switch err := commonpassword.Validate(input.NewPassword); {
	case errors.Is(err, commonpassword.ErrTooShort):
		fields["newPassword"] = ValidationPasswordTooShort
	case errors.Is(err, commonpassword.ErrTooLong):
		fields["newPassword"] = ValidationPasswordTooLong
	}
	return fields
}

// normalizePreferencesInput 规范化并校验语言和时区设置。
func normalizePreferencesInput(input PreferencesInput) (PreferencesInput, map[string]ValidationCode) {
	input.TimeZone = strings.TrimSpace(input.TimeZone)
	fields := make(map[string]ValidationCode)
	if input.Locale != domain.LocaleChineseSimplified && input.Locale != domain.LocaleEnglishUnitedStates {
		fields["locale"] = ValidationLocaleInvalid
	}
	if !commontimezone.Valid(input.TimeZone) {
		fields["timeZone"] = ValidationTimeZoneInvalid
	}
	return input, fields
}
