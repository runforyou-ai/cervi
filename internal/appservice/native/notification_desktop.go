//go:build !server && ((darwin && !ios) || windows || (linux && !android))

package native

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// notificationProvider 使用 Wails 通知服务提供桌面端原生消息提醒。
type notificationProvider struct {
	service *notifications.NotificationService
	ready   atomic.Bool
}

// notificationLifecycle 管理 Wails 通知服务生命周期。
type notificationLifecycle struct {
	service  *notifications.NotificationService
	provider *notificationProvider
}

// NewNotificationProvider 创建原生通知能力及其 Wails 生命周期服务。
func NewNotificationProvider() (appservice.NativeNotification, []application.Service) {
	service := notifications.New()
	provider := &notificationProvider{service: service}
	lifecycle := &notificationLifecycle{service: service, provider: provider}
	return provider, []application.Service{application.NewService(lifecycle)}
}

// ServiceStartup 初始化当前系统的原生通知后端。
func (l *notificationLifecycle) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if err := l.service.ServiceStartup(ctx, options); err != nil {
		l.provider.ready.Store(false)
		slog.Warn("初始化桌面通知服务失败，应用将继续启动", "error", err)
		if shutdownErr := l.service.ServiceShutdown(); shutdownErr != nil {
			slog.Warn("清理未完成初始化的桌面通知服务失败", "error", shutdownErr)
		}
		return nil
	}
	l.provider.ready.Store(true)
	slog.Info("桌面通知服务已初始化")
	return nil
}

// ServiceShutdown 释放当前系统的原生通知后端。
func (l *notificationLifecycle) ServiceShutdown() error {
	if !l.provider.ready.Load() {
		return nil
	}
	l.provider.ready.Store(false)
	if err := l.service.ServiceShutdown(); err != nil {
		slog.Warn("关闭桌面通知服务失败", "error", err)
		return err
	}
	return nil
}

// CheckNotificationPermission 检查当前桌面系统的通知授权状态。
func (p *notificationProvider) CheckNotificationPermission(_ context.Context, _ appservice.RequestMeta) (appservice.NotificationPermissionStatus, error) {
	if !p.ready.Load() {
		return appservice.NotificationPermissionStatusUnsupported, nil
	}
	authorized, err := p.service.CheckNotificationAuthorization()
	if err != nil {
		slog.Warn("检查桌面通知权限失败", "error", err)
		return "", err
	}
	// 转换桌面通知授权状态。
	if authorized {
		return appservice.NotificationPermissionStatusGranted, nil
	}
	return appservice.NotificationPermissionStatusPrompt, nil
}

// RequestNotificationPermission 请求当前桌面系统允许发送通知。
func (p *notificationProvider) RequestNotificationPermission(_ context.Context, _ appservice.RequestMeta) (appservice.NotificationPermissionStatus, error) {
	if !p.ready.Load() {
		return appservice.NotificationPermissionStatusUnsupported, nil
	}
	authorized, err := p.service.RequestNotificationAuthorization()
	if err != nil {
		slog.Warn("申请桌面通知权限失败", "error", err)
		return "", err
	}
	// 转换桌面通知授权结果。
	status := appservice.NotificationPermissionStatusDenied
	if authorized {
		status = appservice.NotificationPermissionStatusGranted
	}
	slog.Info("桌面通知权限申请完成", "status", status)
	return status, nil
}

// SendMessageNotification 投递一条新消息通知。
func (p *notificationProvider) SendMessageNotification(_ context.Context, _ appservice.RequestMeta, input appservice.MessageNotificationInput) error {
	if !p.ready.Load() {
		return errors.New("notification service unavailable")
	}
	// 创建桌面通知参数。
	options := notifications.NotificationOptions{ID: input.ID, Title: input.Title, Body: input.Body}
	if !input.SoundEnabled {
		options.Sound = &notifications.NotificationSound{Silent: true}
	}
	err := p.service.SendNotification(options)
	if err != nil {
		slog.Warn("投递桌面通知失败", "notification_id", input.ID, "sound_enabled", input.SoundEnabled, "error", err)
	}
	return err
}
