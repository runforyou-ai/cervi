//go:build server

package device

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识设备字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationInstallIDRequired     ValidationCode = "DEVICE_INSTALL_ID_REQUIRED"
	ValidationInstallIDTooLong      ValidationCode = "DEVICE_INSTALL_ID_TOO_LONG"
	ValidationNameRequired          ValidationCode = "DEVICE_NAME_REQUIRED"
	ValidationNameTooLong           ValidationCode = "DEVICE_NAME_TOO_LONG"
	ValidationPlatformInvalid       ValidationCode = "DEVICE_PLATFORM_INVALID"
	ValidationRuntimeVersionInvalid ValidationCode = "DEVICE_RUNTIME_VERSION_INVALID"
)

// ValidationError 表示设备字段校验失败。
type ValidationError = common.FieldError

const (
	// maxInstallIDLength 是客户端安装标识的最大字符数。
	maxInstallIDLength = 64
	// maxNameLength 是设备名称的最大字符数。
	maxNameLength = 100
)

// normalizeRegisterInput 归一化并校验设备注册输入。
func normalizeRegisterInput(input RegisterInput) (RegisterInput, map[string]ValidationCode) {
	input.InstallID = strings.TrimSpace(input.InstallID)
	input.Name = strings.TrimSpace(input.Name)
	fields := make(map[string]ValidationCode)
	if input.InstallID == "" {
		fields["installId"] = ValidationInstallIDRequired
	} else if utf8.RuneCountInString(input.InstallID) > maxInstallIDLength {
		fields["installId"] = ValidationInstallIDTooLong
	}
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if utf8.RuneCountInString(input.Name) > maxNameLength {
		fields["name"] = ValidationNameTooLong
	}
	if !domain.ValidDevicePlatform(input.Platform) {
		fields["platform"] = ValidationPlatformInvalid
	}
	if input.RuntimeVersion < 0 {
		fields["runtimeVersion"] = ValidationRuntimeVersionInvalid
	}
	// 工具清单去掉空名称与重复名称，未上报时为空清单。
	manifest := make([]string, 0, len(input.ToolManifest))
	for _, name := range input.ToolManifest {
		if name = strings.TrimSpace(name); name != "" && !slices.Contains(manifest, name) {
			manifest = append(manifest, name)
		}
	}
	input.ToolManifest = manifest
	return input, fields
}
