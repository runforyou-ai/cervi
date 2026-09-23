//go:build server

// Package device 实现成员本机设备的注册、查询与撤销。
package device

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// RegisterInput 定义设备注册上报的本机信息、本机运行时版本与本机工具名称清单。
type RegisterInput struct {
	InstallID      string
	Name           string
	Platform       domain.DevicePlatform
	RuntimeVersion int
	ToolManifest   []string
}

// Record 定义设备记录。
type Record struct {
	ID        string
	Name      string
	Platform  domain.DevicePlatform
	CreatedAt time.Time
	UpdatedAt time.Time
}
