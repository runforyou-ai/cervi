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

// ValidationError 返回联系人字段校验结果。
type ValidationError struct {
	Fields map[string]ValidationCode
}

// Error 返回联系人输入校验错误。
func (e *ValidationError) Error() string {
	return "contact validation failed"
}

func normalizeContactInput(input ContactInput, channelRequired bool) (ContactInput, map[string]ValidationCode) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.Stage = strings.TrimSpace(input.Stage)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.Stage == "" {
		input.Stage = StageVisitor
	}

	fields := make(map[string]ValidationCode)
	if channelRequired && input.ChannelID == "" {
		fields["channelId"] = ValidationChannelRequired
	} else if input.ChannelID != "" && !validUUID(input.ChannelID) {
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
	}

	seen := make(map[string]struct{})
	primarySeen := make(map[string]bool)
	firstByType := make(map[string]int)
	for index := range input.Methods {
		method := &input.Methods[index]
		method.Type = strings.TrimSpace(method.Type)
		method.Value = strings.TrimSpace(method.Value)
		method.Label = strings.TrimSpace(method.Label)
		if len([]rune(method.Label)) > 100 {
			fields["methods"] = ValidationMethodInvalid
		}

		normalized, ok := normalizeMethodValue(method.Type, method.Value)
		if !ok {
			fields["methods"] = ValidationMethodInvalid
			continue
		}
		method.Value = normalized
		key := method.Type + "\x00" + normalized
		if _, exists := seen[key]; exists {
			fields["methods"] = ValidationMethodDuplicate
		}
		seen[key] = struct{}{}
		if _, exists := firstByType[method.Type]; !exists {
			firstByType[method.Type] = index
		}
		if method.IsPrimary {
			if primarySeen[method.Type] {
				fields["methods"] = ValidationPrimaryDuplicate
			}
			primarySeen[method.Type] = true
		}
	}
	for methodType, index := range firstByType {
		if !primarySeen[methodType] {
			input.Methods[index].IsPrimary = true
		}
	}
	if input.DisplayName == "" && len(input.Methods) == 0 {
		fields["displayName"] = ValidationIdentityRequired
	}
	return input, fields
}

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

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && strings.EqualFold(parsed.String(), value)
}
