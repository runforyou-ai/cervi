//go:build server

package integrationconnection

import (
	"strings"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识连接器字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationTypeInvalid        ValidationCode = "INTEGRATION_CONNECTION_TYPE_INVALID"
	ValidationNameRequired       ValidationCode = "INTEGRATION_CONNECTION_NAME_REQUIRED"
	ValidationNameTooLong        ValidationCode = "INTEGRATION_CONNECTION_NAME_TOO_LONG"
	ValidationNameDuplicate      ValidationCode = "INTEGRATION_CONNECTION_NAME_DUPLICATE"
	ValidationDescriptionTooLong ValidationCode = "INTEGRATION_CONNECTION_DESCRIPTION_TOO_LONG"
	ValidationAPIKeyRequired     ValidationCode = "INTEGRATION_CONNECTION_API_KEY_REQUIRED"
	ValidationAPIKeyTooLong      ValidationCode = "INTEGRATION_CONNECTION_API_KEY_TOO_LONG"
	ValidationAPIURLRequired     ValidationCode = "INTEGRATION_CONNECTION_API_URL_REQUIRED"
	ValidationAPIURLInvalid      ValidationCode = "INTEGRATION_CONNECTION_API_URL_INVALID"
)

const (
	// maxNameLength 是连接名称的最大字符数。
	maxNameLength = 100
	// maxDescriptionLength 是连接描述的最大字符数。
	maxDescriptionLength = 2000
	// maxAPIKeyBytes 是 API 密钥的最大字节数。
	maxAPIKeyBytes = 2048
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
	} else if utf8.RuneCountInString(input.Name) > maxNameLength {
		fields["name"] = ValidationNameTooLong
	}
	if utf8.RuneCountInString(input.Description) > maxDescriptionLength {
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
	} else if !common.ValidHTTPBaseURL(input.Configuration.APIURL) {
		fields["configuration.apiUrl"] = ValidationAPIURLInvalid
	}
	if input.Configuration.APIKey == "" {
		fields["configuration.apiKey"] = ValidationAPIKeyRequired
	} else if len(input.Configuration.APIKey) > maxAPIKeyBytes {
		fields["configuration.apiKey"] = ValidationAPIKeyTooLong
	}
	return input, fields
}
