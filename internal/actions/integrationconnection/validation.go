//go:build server

package integrationconnection

import (
	"net/url"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识连接器字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationTypeInvalid        ValidationCode = "TYPE_INVALID"
	ValidationNameRequired       ValidationCode = "NAME_REQUIRED"
	ValidationNameTooLong        ValidationCode = "NAME_TOO_LONG"
	ValidationNameDuplicate      ValidationCode = "NAME_DUPLICATE"
	ValidationDescriptionTooLong ValidationCode = "DESCRIPTION_TOO_LONG"
	ValidationAPIKeyRequired     ValidationCode = "API_KEY_REQUIRED"
	ValidationAPIKeyTooLong      ValidationCode = "API_KEY_TOO_LONG"
	ValidationAPIURLRequired     ValidationCode = "API_URL_REQUIRED"
	ValidationAPIURLInvalid      ValidationCode = "API_URL_INVALID"
)

// ValidationError 表示连接器字段校验失败。
type ValidationError = common.FieldError

// normalizeInput 规范化并校验连接器输入。
func normalizeInput(input Input) (Input, map[string]ValidationCode) {
	fields := make(map[string]ValidationCode)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	connection, connectionFields := normalizeConnectionInput(ConnectionInput{
		Type: input.Type, Configuration: input.Configuration,
	})
	input.Type = connection.Type
	input.Configuration = connection.Configuration
	for field, code := range connectionFields {
		fields[field] = code
	}
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if len([]rune(input.Name)) > 100 {
		fields["name"] = ValidationNameTooLong
	}
	if len([]rune(input.Description)) > 2000 {
		fields["description"] = ValidationDescriptionTooLong
	}
	return input, fields
}

// normalizeConnectionInput 规范化并校验连接器草稿配置。
func normalizeConnectionInput(input ConnectionInput) (ConnectionInput, map[string]ValidationCode) {
	fields := make(map[string]ValidationCode)
	input.Configuration.APIURL = strings.TrimSpace(input.Configuration.APIURL)
	input.Configuration.APIKey = strings.TrimSpace(input.Configuration.APIKey)
	if input.Type != domain.IntegrationConnectionTypeDify &&
		input.Type != domain.IntegrationConnectionTypeN8N {
		fields["type"] = ValidationTypeInvalid
	}
	if input.Configuration.APIURL == "" {
		fields["configuration.apiUrl"] = ValidationAPIURLRequired
	} else if !validAPIURL(input.Configuration.APIURL) {
		fields["configuration.apiUrl"] = ValidationAPIURLInvalid
	}
	if input.Configuration.APIKey == "" {
		fields["configuration.apiKey"] = ValidationAPIKeyRequired
	} else if len(input.Configuration.APIKey) > 2048 {
		fields["configuration.apiKey"] = ValidationAPIKeyTooLong
	}
	return input, fields
}

// validAPIURL 校验地址为不含认证信息的完整 HTTP 地址。
func validAPIURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.IsAbs() && parsed.Host != "" && parsed.User == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.RawQuery == "" && parsed.Fragment == ""
}
