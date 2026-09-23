package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// DevicePlatform 定义注册设备的运行平台。
type DevicePlatform string

const (
	DevicePlatformMacOS   DevicePlatform = DevicePlatform(domain.DevicePlatformMacOS)
	DevicePlatformWindows DevicePlatform = DevicePlatform(domain.DevicePlatformWindows)
	DevicePlatformLinux   DevicePlatform = DevicePlatform(domain.DevicePlatformLinux)
)

// Device 定义成员注册到企业的本机设备。
type Device struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Platform  DevicePlatform `json:"platform"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// DeviceRegistrationInput 定义设备注册上报的本机信息、本机运行时版本与本机工具名称清单。
type DeviceRegistrationInput struct {
	InstallID      string         `json:"installId"`
	Name           string         `json:"name"`
	Platform       DevicePlatform `json:"platform"`
	RuntimeVersion int            `json:"runtimeVersion"`
	ToolManifest   []string       `json:"toolManifest"`
}

// DeviceList 定义当前用户的设备列表。
type DeviceList struct {
	Devices []Device `json:"devices"`
}

// LocalDevice 定义本机在当前企业服务器上的设备注册状态，设备编号为空表示尚未注册。
type LocalDevice struct {
	DeviceID string `json:"deviceId"`
}

// DeviceWorkspaceInput 定义设备工作区的显示名。
type DeviceWorkspaceInput struct {
	Label string `json:"label"`
}

// DeviceWorkspace 定义设备上供 Agent 执行本机工具的工作区。
type DeviceWorkspace struct {
	ID         string     `json:"id"`
	DeviceID   string     `json:"deviceId"`
	Label      string     `json:"label"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// DeviceWorkspaceList 定义设备上的工作区列表。
type DeviceWorkspaceList struct {
	Workspaces []DeviceWorkspace `json:"workspaces"`
}
