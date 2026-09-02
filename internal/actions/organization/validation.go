//go:build server

// Package organization 实现企业查询和通用设置修改。
package organization

import (
	"github.com/runforyou-ai/cervi/internal/common"
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
