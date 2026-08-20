//go:build server

package installation

import (
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
)

// ValidationCode 标识企业初始化字段的校验结果。
type ValidationCode = common.FieldCode

const maxOrganizationNameLength = 32

const (
	ValidationOrganizationNameRequired ValidationCode = "ORGANIZATION_NAME_REQUIRED"
	ValidationOrganizationNameTooLong  ValidationCode = "ORGANIZATION_NAME_TOO_LONG"
	ValidationDisplayNameRequired      ValidationCode = "DISPLAY_NAME_REQUIRED"
	ValidationEmailInvalid             ValidationCode = "EMAIL_INVALID"
	ValidationPasswordTooShort         ValidationCode = "PASSWORD_TOO_SHORT"
	ValidationPasswordTooLong          ValidationCode = "PASSWORD_TOO_LONG"
)

// ValidationError 表示企业初始化字段校验失败。
type ValidationError = common.FieldError

// validateInput 校验企业初始化字段。
func validateInput(input InstallWorkspaceInput) map[string]ValidationCode {
	fields := make(map[string]ValidationCode)
	if input.OrganizationName == "" {
		fields["organizationName"] = ValidationOrganizationNameRequired
	} else if len([]rune(input.OrganizationName)) > maxOrganizationNameLength {
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
	return fields
}
