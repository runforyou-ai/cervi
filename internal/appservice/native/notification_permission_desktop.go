//go:build !server && ((darwin && !ios) || windows || (linux && !android))

package native

import "github.com/runforyou-ai/cervi/internal/appservice"

// checkedNotificationPermission 转换桌面通知授权状态。
func checkedNotificationPermission(authorized bool) appservice.NotificationPermissionStatus {
	if authorized {
		return appservice.NotificationPermissionStatusGranted
	}
	return appservice.NotificationPermissionStatusPrompt
}

// requestedNotificationPermission 转换桌面通知授权结果。
func requestedNotificationPermission(authorized bool) appservice.NotificationPermissionStatus {
	if authorized {
		return appservice.NotificationPermissionStatusGranted
	}
	return appservice.NotificationPermissionStatusDenied
}
