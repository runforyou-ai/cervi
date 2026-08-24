//go:build !server && darwin && !ios

package native

import "github.com/runforyou-ai/cervi/internal/appservice"

// checkedNotificationPermission 将 macOS 当前授权结果转换为应用状态。
func checkedNotificationPermission(authorized bool) appservice.NotificationPermissionStatus {
	if authorized {
		return appservice.NotificationPermissionStatusGranted
	}
	return appservice.NotificationPermissionStatusPrompt
}

// requestedNotificationPermission 将 macOS 授权请求结果转换为应用状态。
func requestedNotificationPermission(authorized bool) appservice.NotificationPermissionStatus {
	if authorized {
		return appservice.NotificationPermissionStatusGranted
	}
	return appservice.NotificationPermissionStatusDenied
}
