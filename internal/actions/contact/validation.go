//go:build server

// Package contact 实现外部联系人领域的应用操作。
package contact

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识联系人字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationIdentityRequired ValidationCode = "CONTACT_IDENTITY_REQUIRED"
	ValidationChannelRequired  ValidationCode = "CONTACT_CHANNEL_REQUIRED"
	ValidationChannelInvalid   ValidationCode = "CONTACT_CHANNEL_INVALID"
	ValidationChannelImmutable ValidationCode = "CONTACT_CHANNEL_IMMUTABLE"
	ValidationNameTooLong      ValidationCode = "CONTACT_NAME_TOO_LONG"
	ValidationStageInvalid     ValidationCode = "CONTACT_STAGE_INVALID"
	ValidationNotesTooLong     ValidationCode = "CONTACT_NOTES_TOO_LONG"
	ValidationMethodsTooMany   ValidationCode = "CONTACT_METHODS_TOO_MANY"
	ValidationMethodInvalid    ValidationCode = "CONTACT_METHOD_INVALID"
	ValidationMethodDuplicate  ValidationCode = "CONTACT_METHOD_DUPLICATE"
	ValidationPrimaryDuplicate ValidationCode = "CONTACT_PRIMARY_DUPLICATE"
	ValidationQueryInvalid     ValidationCode = "CONTACT_QUERY_INVALID"
)

const (
	// maxDisplayNameLength 是联系人名称的最大字符数。
	maxDisplayNameLength = 200
	// maxNotesLength 是联系人备注的最大字符数。
	maxNotesLength = 5000
	// maxMethods 是联系方式的最大数量。
	maxMethods = 20
	// maxMethodLabelLength 是联系方式标签的最大字符数。
	maxMethodLabelLength = 100
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
	if utf8.RuneCountInString(input.DisplayName) > maxDisplayNameLength {
		fields["displayName"] = ValidationNameTooLong
	}
	if input.Stage != domain.ContactStageVisitor && input.Stage != domain.ContactStageLead && input.Stage != domain.ContactStageCustomer {
		fields["stage"] = ValidationStageInvalid
	}
	if utf8.RuneCountInString(input.Notes) > maxNotesLength {
		fields["notes"] = ValidationNotesTooLong
	}
	if len(input.Methods) > maxMethods {
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
		if utf8.RuneCountInString(method.Label) > maxMethodLabelLength && code == "" {
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
	var pageValid bool
	input.Page, input.PageSize, pageValid = common.NormalizePagination(input.Page, input.PageSize)

	fields := make(map[string]ValidationCode)
	if !pageValid {
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
