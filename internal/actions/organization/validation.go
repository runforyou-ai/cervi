//go:build server

// Package organization 实现企业通用设置修改。
package organization

import (
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识企业通用设置的校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationNameRequired ValidationCode = "ORGANIZATION_NAME_REQUIRED"
	ValidationNameTooLong  ValidationCode = "ORGANIZATION_NAME_TOO_LONG"
)

// ValidationError 表示企业通用设置校验失败。
type ValidationError = common.FieldError

// Input 定义企业通用设置修改输入。
type Input struct {
	Name              string
	AllowArbitraryURL bool
}

// normalizeInput 归一化并校验企业通用设置。
func normalizeInput(input Input) (Input, map[string]ValidationCode) {
	input.Name = strings.TrimSpace(input.Name)
	fields := make(map[string]ValidationCode)
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if len([]rune(input.Name)) > domain.OrganizationNameMaxLength {
		fields["name"] = ValidationNameTooLong
	}
	return input, fields
}
