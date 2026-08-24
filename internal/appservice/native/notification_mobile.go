//go:build !server && (android || ios)

package native

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// NotificationProvider 为本期未接入通知的移动端保留统一能力占位。
type NotificationProvider struct{}

// NewNotificationProvider 创建不需要注册 Wails 生命周期服务的移动端占位实现。
func NewNotificationProvider() (*NotificationProvider, []application.Service) {
	return &NotificationProvider{}, nil
}

// CheckNotificationPermission 返回移动端本期不支持通知的状态。
func (*NotificationProvider) CheckNotificationPermission(_ context.Context, _ appservice.RequestMeta) (appservice.NotificationPermissionStatus, error) {
	return appservice.NotificationPermissionStatusUnsupported, nil
}

// RequestNotificationPermission 返回移动端本期不支持通知的状态。
func (*NotificationProvider) RequestNotificationPermission(_ context.Context, _ appservice.RequestMeta) (appservice.NotificationPermissionStatus, error) {
	return appservice.NotificationPermissionStatusUnsupported, nil
}

// SendMessageNotification 在移动端本期保持为空操作。
func (*NotificationProvider) SendMessageNotification(_ context.Context, _ appservice.RequestMeta, _ appservice.MessageNotificationInput) error {
	return nil
}

// SendTestNotification 在移动端本期保持为空操作。
func (*NotificationProvider) SendTestNotification(_ context.Context, _ appservice.RequestMeta, _ appservice.NotificationSoundInput) error {
	return nil
}

// SetUnreadIndicator 在移动端本期保持为空操作。
func (*NotificationProvider) SetUnreadIndicator(_ appservice.UnreadIndicator) {}

// UpdateUnreadIndicator 在移动端本期保持为空操作。
func (*NotificationProvider) UpdateUnreadIndicator(_ context.Context, _ appservice.RequestMeta, _ appservice.UnreadIndicatorState) error {
	return nil
}
