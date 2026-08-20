//go:build server

package installation

import (
	"errors"

	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	"github.com/runforyou-ai/cervi/internal/common/fielderror"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
)

// ValidationCode 标识企业初始化字段的校验结果。
type ValidationCode = fielderror.Code

const (
	ValidationOrganizationNameRequired ValidationCode = "ORGANIZATION_NAME_REQUIRED"
	ValidationDisplayNameRequired      ValidationCode = "DISPLAY_NAME_REQUIRED"
	ValidationEmailInvalid             ValidationCode = "EMAIL_INVALID"
	ValidationPasswordTooShort         ValidationCode = "PASSWORD_TOO_SHORT"
	ValidationPasswordTooLong          ValidationCode = "PASSWORD_TOO_LONG"
)

// ValidationError 表示企业初始化字段校验失败。
type ValidationError = fielderror.Error

// validateInput 校验企业初始化字段。
func validateInput(input InstallWorkspaceInput) map[string]ValidationCode {
	fields := make(map[string]ValidationCode)
	if input.OrganizationName == "" {
		fields["organizationName"] = ValidationOrganizationNameRequired
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
