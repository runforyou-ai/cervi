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
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Platform       DevicePlatform `json:"platform"`
	RuntimeVersion string         `json:"runtimeVersion"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// DeviceRegistrationInput 定义设备注册上报的本机信息。
type DeviceRegistrationInput struct {
	InstallID      string         `json:"installId"`
	Name           string         `json:"name"`
	Platform       DevicePlatform `json:"platform"`
	RuntimeVersion string         `json:"runtimeVersion"`
}

// DeviceList 定义当前用户的设备列表。
type DeviceList struct {
	Devices []Device `json:"devices"`
}

// LocalDevice 定义本机在当前企业服务器上的设备注册状态，设备编号为空表示尚未注册。
type LocalDevice struct {
	DeviceID string `json:"deviceId"`
}
