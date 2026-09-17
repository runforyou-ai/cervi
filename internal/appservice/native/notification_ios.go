//go:build !server && ios

package native

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UserNotifications
#include <stdlib.h>
#include "notification_ios.h"
*/
import "C"

import (
	"context"
	"errors"
	"log/slog"
	"unsafe"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// notificationProvider 通过系统通知中心提供 iOS 本地通知能力。
type notificationProvider struct{}

// NewNotificationProvider 创建 iOS 原生通知能力。
func NewNotificationProvider() (appservice.NativeNotification, []application.Service) {
	return notificationProvider{}, nil
}

// notificationPermissionStatus 把原生授权取值转换为应用服务的授权状态。
func notificationPermissionStatus(status C.int) appservice.NotificationPermissionStatus {
	switch status {
	case C.CERVI_NOTIFICATION_STATUS_PROMPT:
		return appservice.NotificationPermissionStatusPrompt
	case C.CERVI_NOTIFICATION_STATUS_GRANTED:
		return appservice.NotificationPermissionStatusGranted
	case C.CERVI_NOTIFICATION_STATUS_DENIED:
		return appservice.NotificationPermissionStatusDenied
	default:
		return appservice.NotificationPermissionStatusUnsupported
	}
}

// CheckNotificationPermission 检查当前设备的通知授权状态。
func (notificationProvider) CheckNotificationPermission(_ context.Context, _ appservice.RequestMeta) (appservice.NotificationPermissionStatus, error) {
	return notificationPermissionStatus(C.cervi_notification_authorization_status()), nil
}

// RequestNotificationPermission 申请当前设备的通知授权。
func (notificationProvider) RequestNotificationPermission(_ context.Context, _ appservice.RequestMeta) (appservice.NotificationPermissionStatus, error) {
	status := notificationPermissionStatus(C.cervi_notification_request_authorization())
	slog.Info("移动端通知权限申请完成", "status", status)
	return status, nil
}

// SendMessageNotification 投递一条新消息通知。
func (notificationProvider) SendMessageNotification(_ context.Context, _ appservice.RequestMeta, input appservice.MessageNotificationInput) error {
	identifier := C.CString(input.ID)
	defer C.free(unsafe.Pointer(identifier))
	title := C.CString(input.Title)
	defer C.free(unsafe.Pointer(title))
	body := C.CString(input.Body)
	defer C.free(unsafe.Pointer(body))
	// 通知声音由本机偏好控制，关闭时投递静音通知。
	silent := C.int(1)
	if input.SoundEnabled {
		silent = C.int(0)
	}
	if C.cervi_notification_post(identifier, title, body, silent) != 0 {
		slog.Warn("投递移动端通知失败", "notification_id", input.ID, "sound_enabled", input.SoundEnabled)
		return errors.New("post notification failed")
	}
	return nil
}
