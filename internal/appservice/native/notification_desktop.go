//go:build !server && ((darwin && !ios) || windows || (linux && !android))

package native

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// NotificationProvider 使用 Wails 通知服务提供桌面端原生消息提醒。
type NotificationProvider struct {
	service *notifications.NotificationService

	availabilityMu sync.RWMutex
	ready          bool
	startupErr     error

	unreadMu        sync.Mutex
	unreadIndicator appservice.UnreadIndicator
}

// notificationLifecycle 只负责把 Wails 生命周期转发给底层通知服务。
type notificationLifecycle struct {
	service  *notifications.NotificationService
	provider *NotificationProvider
}

// NewNotificationProvider 创建原生通知能力及其 Wails 生命周期服务。
func NewNotificationProvider() (*NotificationProvider, []application.Service) {
	service := notifications.New()
	provider := &NotificationProvider{service: service}
	lifecycle := &notificationLifecycle{service: service, provider: provider}
	return provider, []application.Service{application.NewService(lifecycle)}
}

// ServiceStartup 初始化当前系统的原生通知后端。
func (l *notificationLifecycle) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if err := l.service.ServiceStartup(ctx, options); err != nil {
		l.provider.setAvailability(false, err)
		slog.Warn("初始化桌面通知服务失败，应用将继续启动", "error", err)
		if shutdownErr := l.service.ServiceShutdown(); shutdownErr != nil {
			slog.Warn("清理未完成初始化的桌面通知服务失败", "error", shutdownErr)
		}
		return nil
	}
	l.provider.setAvailability(true, nil)
	return nil
}

// ServiceShutdown 释放当前系统的原生通知后端。
func (l *notificationLifecycle) ServiceShutdown() error {
	if !l.provider.isReady() {
		return nil
	}
	defer l.provider.setAvailability(false, nil)
	return l.service.ServiceShutdown()
}

// CheckNotificationPermission 检查当前桌面系统的通知授权状态。
func (p *NotificationProvider) CheckNotificationPermission(_ context.Context, _ appservice.RequestMeta) (appservice.NotificationPermissionStatus, error) {
	if !p.isReady() {
		return appservice.NotificationPermissionStatusUnsupported, nil
	}
	authorized, err := p.service.CheckNotificationAuthorization()
	if err != nil {
		return "", err
	}
	return checkedNotificationPermission(authorized), nil
}

// RequestNotificationPermission 请求当前桌面系统允许发送通知。
func (p *NotificationProvider) RequestNotificationPermission(_ context.Context, _ appservice.RequestMeta) (appservice.NotificationPermissionStatus, error) {
	if !p.isReady() {
		return appservice.NotificationPermissionStatusUnsupported, nil
	}
	authorized, err := p.service.RequestNotificationAuthorization()
	if err != nil {
		return "", err
	}
	return requestedNotificationPermission(authorized), nil
}

// SendMessageNotification 投递一条使用系统默认声音或静音的新消息通知。
func (p *NotificationProvider) SendMessageNotification(_ context.Context, _ appservice.RequestMeta, input appservice.MessageNotificationInput) error {
	if err := p.availabilityError(); err != nil {
		return err
	}
	return p.service.SendNotification(notificationOptions(
		input.ID,
		input.Title,
		input.Body,
		input.SoundEnabled,
	))
}

// notificationOptions 组装使用系统默认声音或静音的通知参数。
func notificationOptions(id string, title string, body string, soundEnabled bool) notifications.NotificationOptions {
	options := notifications.NotificationOptions{ID: id, Title: title, Body: body}
	if !soundEnabled {
		options.Sound = &notifications.NotificationSound{Silent: true}
	}
	return options
}

// SetUnreadIndicator 连接应用创建后可用的原生托盘与角标实现。
func (p *NotificationProvider) SetUnreadIndicator(indicator appservice.UnreadIndicator) {
	p.unreadMu.Lock()
	defer p.unreadMu.Unlock()
	p.unreadIndicator = indicator
}

// UpdateUnreadIndicator 将最新未读状态同步到原生托盘与角标。
func (p *NotificationProvider) UpdateUnreadIndicator(_ context.Context, _ appservice.RequestMeta, state appservice.UnreadIndicatorState) error {
	p.unreadMu.Lock()
	defer p.unreadMu.Unlock()
	if p.unreadIndicator == nil {
		return nil
	}
	return p.unreadIndicator.SetUnreadState(state)
}

// setAvailability 保存桌面通知后端是否可用及初始化错误。
func (p *NotificationProvider) setAvailability(ready bool, startupErr error) {
	p.availabilityMu.Lock()
	defer p.availabilityMu.Unlock()
	p.ready = ready
	p.startupErr = startupErr
}

// isReady 返回桌面通知后端是否已经成功初始化。
func (p *NotificationProvider) isReady() bool {
	p.availabilityMu.RLock()
	defer p.availabilityMu.RUnlock()
	return p.ready
}

// availabilityError 返回桌面通知后端不可用的可诊断错误。
func (p *NotificationProvider) availabilityError() error {
	p.availabilityMu.RLock()
	defer p.availabilityMu.RUnlock()
	if p.ready {
		return nil
	}
	if p.startupErr != nil {
		return fmt.Errorf("notification service unavailable: %w", p.startupErr)
	}
	return errors.New("notification service unavailable")
}
