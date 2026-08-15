// Package i18n 提供后端用户可见文案的本地化能力。
package i18n

import (
	"embed"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Key 标识一条后端本地化文案。
type Key string

const (
	ErrorMethodNotAllowed             Key = "error.method_not_allowed"
	ErrorInstallationStatusReadFailed Key = "error.installation_status_read_failed"
	ErrorAlreadyInitialized           Key = "error.already_initialized"
	ErrorInstallationRequired         Key = "error.installation_required"
	ErrorAuthenticationStatusFailed   Key = "error.authentication_status_failed"
	ErrorAuthenticationRequired       Key = "error.authentication_required"
	ErrorValidationFailed             Key = "error.validation_failed"
	ErrorInstallationFailed           Key = "error.installation_failed"
	ErrorAuthenticationInputInvalid   Key = "error.authentication_input_invalid"
	ErrorInvalidCredentials           Key = "error.invalid_credentials"
	ErrorLoginFailed                  Key = "error.login_failed"
	ErrorLogoutFailed                 Key = "error.logout_failed"
	ErrorServerURLInvalid             Key = "error.server_url_invalid"
	ErrorServerConnectionCreateFailed Key = "error.server_connection_create_failed"
	ErrorServerUnavailable            Key = "error.server_unavailable"
	ErrorServerConnectionSaveFailed   Key = "error.server_connection_save_failed"
	ErrorServerConnectionRequired     Key = "error.server_connection_required"
	ErrorRemoteRequestCreateFailed    Key = "error.remote_request_create_failed"
	ErrorServerConnectionFailed       Key = "error.server_connection_failed"

	FieldOrganizationNameRequired Key = "field.organization_name_required"
	FieldDisplayNameRequired      Key = "field.display_name_required"
	FieldEmailInvalid             Key = "field.email_invalid"
	FieldPasswordTooShort         Key = "field.password_too_short"
	FieldPasswordTooLong          Key = "field.password_too_long"
	FieldServerURLComplete        Key = "field.server_url_complete"
	FieldServerURLBaseOnly        Key = "field.server_url_base_only"
	FieldServerURLHTTPSRequired   Key = "field.server_url_https_required"
	FieldServerURLNotCervi        Key = "field.server_url_not_cervi"
)

//go:embed locales/*.json
var localeFiles embed.FS

var bundle = loadBundle()

// Localize 根据语言偏好返回本地化文案和最终匹配的语言。
func Localize(acceptLanguage string, key Key) (string, string) {
	localizer := goi18n.NewLocalizer(bundle, acceptLanguage)
	message, tag, err := localizer.LocalizeWithTag(&goi18n.LocalizeConfig{MessageID: string(key)})
	if err != nil {
		panic(err)
	}
	return message, tag.String()
}

// LocalizeMap 将一组命名文案键翻译为对应文案。
func LocalizeMap(acceptLanguage string, keys map[string]Key) map[string]string {
	if len(keys) == 0 {
		return nil
	}
	localizer := goi18n.NewLocalizer(bundle, acceptLanguage)
	messages := make(map[string]string, len(keys))
	for name, key := range keys {
		messages[name] = localizer.MustLocalize(&goi18n.LocalizeConfig{MessageID: string(key)})
	}
	return messages
}

// loadBundle 加载嵌入到二进制中的翻译词条。
func loadBundle() *goi18n.Bundle {
	bundle := goi18n.NewBundle(language.AmericanEnglish)
	if _, err := bundle.LoadMessageFileFS(localeFiles, "locales/en-US.json"); err != nil {
		panic(err)
	}
	if _, err := bundle.LoadMessageFileFS(localeFiles, "locales/zh-CN.json"); err != nil {
		panic(err)
	}
	return bundle
}
