//go:build server

package installation

import (
	"errors"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	commontimezone "github.com/runforyou-ai/cervi/internal/common/timezone"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识企业初始化字段的校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationOrganizationNameRequired ValidationCode = "INSTALLATION_ORGANIZATION_NAME_REQUIRED"
	ValidationOrganizationNameTooLong  ValidationCode = "INSTALLATION_ORGANIZATION_NAME_TOO_LONG"
	ValidationDisplayNameRequired      ValidationCode = "INSTALLATION_DISPLAY_NAME_REQUIRED"
	ValidationEmailInvalid             ValidationCode = "INSTALLATION_EMAIL_INVALID"
	ValidationPasswordTooShort         ValidationCode = "INSTALLATION_PASSWORD_TOO_SHORT"
	ValidationPasswordTooLong          ValidationCode = "INSTALLATION_PASSWORD_TOO_LONG"
	ValidationLocaleInvalid            ValidationCode = "INSTALLATION_LOCALE_INVALID"
	ValidationTimeZoneInvalid          ValidationCode = "INSTALLATION_TIME_ZONE_INVALID"
)

// ValidationError 表示企业初始化字段校验失败。
type ValidationError = common.FieldError

// validateInput 校验企业初始化字段。
func validateInput(input InstallWorkspaceInput) map[string]ValidationCode {
	fields := make(map[string]ValidationCode)
	if input.OrganizationName == "" {
		fields["organizationName"] = ValidationOrganizationNameRequired
	} else if utf8.RuneCountInString(input.OrganizationName) > domain.OrganizationNameMaxLength {
		fields["organizationName"] = ValidationOrganizationNameTooLong
	}
	if input.DisplayName == "" {
		fields["displayName"] = ValidationDisplayNameRequired
	}
	if !commonemail.Valid(input.Email) {
		fields["email"] = ValidationEmailInvalid
	}
	switch err := commonpassword.Validate(input.Password); {
	case errors.Is(err, commonpassword.ErrTooShort):
		fields["password"] = ValidationPasswordTooShort
	case errors.Is(err, commonpassword.ErrTooLong):
		fields["password"] = ValidationPasswordTooLong
	}
	if input.Locale != domain.LocaleChineseSimplified && input.Locale != domain.LocaleEnglishUnitedStates {
		fields["locale"] = ValidationLocaleInvalid
	}
	if !commontimezone.Valid(input.TimeZone) {
		fields["timeZone"] = ValidationTimeZoneInvalid
	}
	return fields
}
