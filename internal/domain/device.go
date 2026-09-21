package domain

// DevicePlatform 定义注册设备的运行平台。
type DevicePlatform string

const (
	DevicePlatformMacOS   DevicePlatform = "macos"
	DevicePlatformWindows DevicePlatform = "windows"
	DevicePlatformLinux   DevicePlatform = "linux"
)

// ValidDevicePlatform 判断设备平台是否为已知取值。
func ValidDevicePlatform(platform DevicePlatform) bool {
	switch platform {
	case DevicePlatformMacOS, DevicePlatformWindows, DevicePlatformLinux:
		return true
	}
	return false
}

// PeerTriggerCapability 定义绑定设备的会话中设备主人以外成员触发 Agent 时的能力等级。
type PeerTriggerCapability string

const (
	PeerTriggerCapabilityOff               PeerTriggerCapability = "off"
	PeerTriggerCapabilityReadOnly          PeerTriggerCapability = "read_only"
	PeerTriggerCapabilityWriteWithApproval PeerTriggerCapability = "write_with_approval"
	PeerTriggerCapabilitySameAsOwner       PeerTriggerCapability = "same_as_owner"
)
