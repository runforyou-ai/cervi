//go:build server

package user

import (
	"errors"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
)

// ValidationCode 标识用户字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationDisplayNameRequired      ValidationCode = "USER_DISPLAY_NAME_REQUIRED"
	ValidationEmailInvalid             ValidationCode = "USER_EMAIL_INVALID"
	ValidationEmailDuplicate           ValidationCode = "USER_EMAIL_DUPLICATE"
	ValidationCurrentPasswordIncorrect ValidationCode = "USER_CURRENT_PASSWORD_INCORRECT"
	ValidationPasswordTooShort         ValidationCode = "USER_PASSWORD_TOO_SHORT"
	ValidationPasswordTooLong          ValidationCode = "USER_PASSWORD_TOO_LONG"
	ValidationLocaleInvalid            ValidationCode = "USER_LOCALE_INVALID"
	ValidationTimeZoneInvalid          ValidationCode = "USER_TIME_ZONE_INVALID"
	ValidationWorkStatusInvalid        ValidationCode = "USER_WORK_STATUS_INVALID"
	ValidationRoleInvalid              ValidationCode = "USER_ROLE_INVALID"
	ValidationTeamInvalid              ValidationCode = "USER_TEAM_INVALID"
	ValidationStatusInvalid            ValidationCode = "USER_STATUS_INVALID"
)

// ValidationError 表示用户字段校验失败。
type ValidationError = common.FieldError

// normalizeCreateInput 规范化并校验新增企业成员字段。
func normalizeCreateInput(input CreateInput) (CreateInput, map[string]ValidationCode) {
	profile, fields := normalizeProfileInput(ProfileInput{DisplayName: input.DisplayName, Email: input.Email})
	input.DisplayName = profile.DisplayName
	input.Email = profile.Email
	var roleIDValid bool
	input.RoleID, roleIDValid = common.NormalizeUUID(input.RoleID)
	if !roleIDValid {
		fields["roleId"] = ValidationRoleInvalid
	}
	switch err := commonpassword.Validate(input.Password); {
	case errors.Is(err, commonpassword.ErrTooShort):
		fields["password"] = ValidationPasswordTooShort
	case errors.Is(err, commonpassword.ErrTooLong):
		fields["password"] = ValidationPasswordTooLong
	}
	return input, fields
}

// normalizeProfileInput 规范化并校验个人资料输入。
func normalizeProfileInput(input ProfileInput) (ProfileInput, map[string]ValidationCode) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = commonemail.Normalize(input.Email)
	input.AvatarFileID = strings.TrimSpace(input.AvatarFileID)

	fields := make(map[string]ValidationCode)
	if input.DisplayName == "" {
		fields["displayName"] = ValidationDisplayNameRequired
	}
	if !commonemail.Valid(input.Email) {
		fields["email"] = ValidationEmailInvalid
	}
	return input, fields
}
