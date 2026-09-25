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

// DeviceRegistrationInput 定义设备注册上报的本机信息。
type DeviceRegistrationInput struct {
	InstallID string         `json:"installId"`
	Name      string         `json:"name"`
	Platform  DevicePlatform `json:"platform"`
}

// DeviceList 定义当前用户的设备列表。
type DeviceList struct {
	Devices []Device `json:"devices"`
}

// LocalDeviceChangedEventName 是原生端本机设备状态变化的 Wails 事件名，事件不携带数据，界面收到后重新读取本机设备。
const LocalDeviceChangedEventName = "cervi:local-device:changed"

// LocalDevice 定义本机在当前企业服务器上的设备注册状态与 Agent 运行环境，设备编号为空表示尚未注册；不执行 Agent 运行的平台运行环境为空。
type LocalDevice struct {
	DeviceID  string          `json:"deviceId"`
	Toolchain *LocalToolchain `json:"toolchain"`
}

// LocalToolchainState 定义本机 Agent 运行环境的准备状态。
type LocalToolchainState string

const (
	// LocalToolchainStatePreparing 表示运行环境正在准备或尚未开始准备。
	LocalToolchainStatePreparing LocalToolchainState = "preparing"
	// LocalToolchainStateReady 表示已有可用的运行环境。
	LocalToolchainStateReady LocalToolchainState = "ready"
	// LocalToolchainStateFailed 表示最近一次准备失败，等待自动重试。
	LocalToolchainStateFailed LocalToolchainState = "failed"
)

// LocalToolchainFailure 定义运行环境准备失败的原因。
type LocalToolchainFailure string

const (
	// LocalToolchainFailureDownload 表示无法从下载源取得安装文件。
	LocalToolchainFailureDownload LocalToolchainFailure = "download"
	// LocalToolchainFailureVerify 表示下载的安装文件校验失败。
	LocalToolchainFailureVerify LocalToolchainFailure = "verify"
	// LocalToolchainFailureInstall 表示在本机安装失败。
	LocalToolchainFailureInstall LocalToolchainFailure = "install"
)

// LocalToolchain 定义本机 Agent 运行环境的准备状态，失败原因只在失败状态下非空。
type LocalToolchain struct {
	State   LocalToolchainState    `json:"state"`
	Failure *LocalToolchainFailure `json:"failure"`
}
