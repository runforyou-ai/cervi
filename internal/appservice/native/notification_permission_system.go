//go:build !server && (windows || (linux && !android))

package native

import "github.com/runforyou-ai/cervi/internal/appservice"

// checkedNotificationPermission 返回由 Windows 或 Linux 系统统一管理的通知权限状态。
func checkedNotificationPermission(_ bool) appservice.NotificationPermissionStatus {
	return appservice.NotificationPermissionStatusSystemManaged
}

// requestedNotificationPermission 返回由 Windows 或 Linux 系统统一管理的通知权限状态。
func requestedNotificationPermission(_ bool) appservice.NotificationPermissionStatus {
	return appservice.NotificationPermissionStatusSystemManaged
}
