//go:build server

// Package contact 实现外部联系人领域的应用操作。
package contact

import (
	"strings"
	"unicode"

	"github.com/google/uuid"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
)

// ValidationCode 标识联系人字段校验结果。
type ValidationCode string

const (
	ValidationIdentityRequired ValidationCode = "IDENTITY_REQUIRED"
	ValidationChannelRequired  ValidationCode = "CHANNEL_REQUIRED"
	ValidationChannelInvalid   ValidationCode = "CHANNEL_INVALID"
	ValidationChannelImmutable ValidationCode = "CHANNEL_IMMUTABLE"
	ValidationNameTooLong      ValidationCode = "NAME_TOO_LONG"
	ValidationStageInvalid     ValidationCode = "STAGE_INVALID"
	ValidationNotesTooLong     ValidationCode = "NOTES_TOO_LONG"
	ValidationMethodsTooMany   ValidationCode = "METHODS_TOO_MANY"
	ValidationMethodInvalid    ValidationCode = "METHOD_INVALID"
	ValidationMethodDuplicate  ValidationCode = "METHOD_DUPLICATE"
	ValidationPrimaryDuplicate ValidationCode = "PRIMARY_DUPLICATE"
	ValidationQueryInvalid     ValidationCode = "QUERY_INVALID"
)

// ValidationError 记录联系人字段校验结果。
type ValidationError struct {
	Fields map[string]ValidationCode
}

// Error 返回联系人校验错误说明。
func (e *ValidationError) Error() string {
	return "contact validation failed"
}

// normalizeContactInput 规范化并校验联系人写入字段。
func normalizeContactInput(input ContactInput) (ContactInput, map[string]ValidationCode) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.Stage = strings.TrimSpace(input.Stage)
	input.Notes = strings.TrimSpace(input.Notes)

	fields := make(map[string]ValidationCode)
	if input.ChannelID == "" {
		fields["channelId"] = ValidationChannelRequired
	} else if !validUUID(input.ChannelID) {
		fields["channelId"] = ValidationChannelInvalid
	}
	if len([]rune(input.DisplayName)) > 200 {
		fields["displayName"] = ValidationNameTooLong
	}
	if input.Stage != StageVisitor && input.Stage != StageLead && input.Stage != StageCustomer {
		fields["stage"] = ValidationStageInvalid
	}
	if len([]rune(input.Notes)) > 5000 {
		fields["notes"] = ValidationNotesTooLong
	}
	if len(input.Methods) > 20 {
		fields["methods"] = ValidationMethodsTooMany
	} else {
		var methodCode ValidationCode
		input.Methods, methodCode = normalizeMethods(input.Methods)
		if methodCode != "" {
			fields["methods"] = methodCode
		}
	}
	if input.DisplayName == "" && len(input.Methods) == 0 {
		fields["displayName"] = ValidationIdentityRequired
	}
	return input, fields
}

// normalizeMethods 规范化联系方式并返回优先级最高的错误码。
func normalizeMethods(methods []MethodInput) ([]MethodInput, ValidationCode) {
	type methodKey struct {
		typeName string
		value    string
	}
	seen := make(map[methodKey]struct{})
	primarySeen := make(map[string]bool)
	firstByType := make(map[string]int)
	var code ValidationCode
	for index := range methods {
		method := &methods[index]
		method.Type = strings.TrimSpace(method.Type)
		method.Value = strings.TrimSpace(method.Value)
		method.Label = strings.TrimSpace(method.Label)
		if len([]rune(method.Label)) > 100 && code == "" {
			code = ValidationMethodInvalid
		}

		normalized, ok := normalizeMethodValue(method.Type, method.Value)
		if !ok {
			if code == "" {
				code = ValidationMethodInvalid
			}
			continue
		}
		method.Value = normalized
		key := methodKey{typeName: method.Type, value: normalized}
		if _, exists := seen[key]; exists {
			if code != ValidationPrimaryDuplicate {
				code = ValidationMethodDuplicate
			}
		}
		seen[key] = struct{}{}
		if _, exists := firstByType[method.Type]; !exists {
			firstByType[method.Type] = index
		}
		if method.IsPrimary {
			if primarySeen[method.Type] {
				code = ValidationPrimaryDuplicate
			}
			primarySeen[method.Type] = true
		}
	}
	for methodType, index := range firstByType {
		if !primarySeen[methodType] {
			methods[index].IsPrimary = true
		}
	}
	return methods, code
}

// normalizeListInput 规范化并校验联系人列表参数。
func normalizeListInput(input ListInput) (ListInput, map[string]ValidationCode) {
	input.Query = strings.TrimSpace(input.Query)
	input.Stage = strings.TrimSpace(input.Stage)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.MethodType = strings.TrimSpace(input.MethodType)
	input.Sort = strings.TrimSpace(input.Sort)
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 50
	}

	fields := make(map[string]ValidationCode)
	if input.PageSize > 100 {
		fields["pageSize"] = ValidationQueryInvalid
	}
	if input.Stage != "" && input.Stage != StageVisitor && input.Stage != StageLead && input.Stage != StageCustomer {
		fields["stage"] = ValidationStageInvalid
	}
	if input.MethodType != "" && input.MethodType != MethodEmail && input.MethodType != MethodPhone {
		fields["methodType"] = ValidationQueryInvalid
	}
	if input.ChannelID != "" && !validUUID(input.ChannelID) {
		fields["channelId"] = ValidationQueryInvalid
	}
	if input.Sort == "" {
		input.Sort = "createdAt.desc"
	}
	if input.Sort != "updatedAt.desc" && input.Sort != "createdAt.desc" && input.Sort != "displayName.asc" {
		fields["sort"] = ValidationQueryInvalid
	}
	return input, fields
}

// normalizeMethodValue 规范化邮箱或国际电话号码。
func normalizeMethodValue(methodType, value string) (string, bool) {
	switch methodType {
	case MethodEmail:
		normalized := commonemail.Normalize(value)
		return normalized, commonemail.Valid(normalized)
	case MethodPhone:
		normalized := strings.Map(func(value rune) rune {
			if (value >= '0' && value <= '9') || value == '+' {
				return value
			}
			if unicode.IsSpace(value) || value == '-' || value == '(' || value == ')' {
				return -1
			}
			return value
		}, value)
		digits := []rune(strings.TrimPrefix(normalized, "+"))
		if !strings.HasPrefix(normalized, "+") || len(digits) < 6 || len(digits) > 15 {
			return "", false
		}
		for _, value := range digits {
			if value >= '0' && value <= '9' {
				continue
			}
			return "", false
		}
		return normalized, true
	default:
		return "", false
	}
}

// validUUID 校验规范化 UUID 字符串。
func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && strings.EqualFold(parsed.String(), value)
}
