//go:build server

// Package contact 实现外部联系人领域的应用操作。
package contact

import (
	"strings"
	"unicode"

	"github.com/runforyou-ai/cervi/internal/common"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识联系人字段校验结果。
type ValidationCode = common.FieldCode

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

// ValidationError 表示联系人字段校验失败。
type ValidationError = common.FieldError

// normalizeContactInput 规范化并校验联系人写入字段。
func normalizeContactInput(input ContactInput) (ContactInput, map[string]ValidationCode) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.Stage = domain.ContactStage(strings.TrimSpace(string(input.Stage)))
	input.Notes = strings.TrimSpace(input.Notes)

	fields := make(map[string]ValidationCode)
	if input.ChannelID == "" {
		fields["channelId"] = ValidationChannelRequired
	} else if !common.ValidUUID(input.ChannelID) {
		fields["channelId"] = ValidationChannelInvalid
	}
	if len([]rune(input.DisplayName)) > 200 {
		fields["displayName"] = ValidationNameTooLong
	}
	if input.Stage != domain.ContactStageVisitor && input.Stage != domain.ContactStageLead && input.Stage != domain.ContactStageCustomer {
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
		method.Type = domain.ContactMethodType(strings.TrimSpace(string(method.Type)))
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
		key := methodKey{typeName: string(method.Type), value: normalized}
		if _, exists := seen[key]; exists {
			if code != ValidationPrimaryDuplicate {
				code = ValidationMethodDuplicate
			}
		}
		seen[key] = struct{}{}
		if _, exists := firstByType[string(method.Type)]; !exists {
			firstByType[string(method.Type)] = index
		}
		if method.IsPrimary {
			if primarySeen[string(method.Type)] {
				code = ValidationPrimaryDuplicate
			}
			primarySeen[string(method.Type)] = true
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
	input.Stage = domain.ContactStage(strings.TrimSpace(string(input.Stage)))
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.MethodType = domain.ContactMethodType(strings.TrimSpace(string(input.MethodType)))
	input.Sort = domain.ContactSort(strings.TrimSpace(string(input.Sort)))
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
	if input.Stage != "" && input.Stage != domain.ContactStageVisitor && input.Stage != domain.ContactStageLead && input.Stage != domain.ContactStageCustomer {
		fields["stage"] = ValidationStageInvalid
	}
	if input.MethodType != "" && input.MethodType != domain.ContactMethodTypeEmail && input.MethodType != domain.ContactMethodTypePhone {
		fields["methodType"] = ValidationQueryInvalid
	}
	if input.ChannelID != "" && !common.ValidUUID(input.ChannelID) {
		fields["channelId"] = ValidationQueryInvalid
	}
	if input.Sort == "" {
		input.Sort = domain.ContactSortCreatedAtDescending
	}
	if input.Sort != domain.ContactSortUpdatedAtDescending && input.Sort != domain.ContactSortCreatedAtDescending && input.Sort != domain.ContactSortDisplayNameAscending {
		fields["sort"] = ValidationQueryInvalid
	}
	return input, fields
}

// normalizeMethodValue 规范化邮箱或国际电话号码。
func normalizeMethodValue(methodType domain.ContactMethodType, value string) (string, bool) {
	switch methodType {
	case domain.ContactMethodTypeEmail:
		normalized := commonemail.Normalize(value)
		return normalized, commonemail.Valid(normalized)
	case domain.ContactMethodTypePhone:
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
