package appservice

import (
	"context"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// Service 将跨平台业务调用转发给当前运行平台的 Backend。
type Service struct {
	backend             Backend
	imageSelector       ImageSelector
	nativeLocaleUpdater NativeLocaleUpdater
	nativeNotification  NativeNotification
	unreadIndicator     UnreadIndicator
	externalPageOpener  ExternalPageOpener
}

// Option 配置平台专属的应用服务能力。
type Option func(*Service)

// WithImageSelector 注入原生端图片文件选择器。
func WithImageSelector(selector ImageSelector) Option {
	return func(service *Service) {
		service.imageSelector = selector
	}
}

// WithNativeLocaleUpdater 注入原生界面语言同步能力。
func WithNativeLocaleUpdater(updater NativeLocaleUpdater) Option {
	return func(service *Service) {
		service.nativeLocaleUpdater = updater
	}
}

// WithNativeNotification 注入原生端系统通知能力。
func WithNativeNotification(notification NativeNotification) Option {
	return func(service *Service) {
		service.nativeNotification = notification
	}
}

// WithUnreadIndicator 注入原生端未读提示能力。
func WithUnreadIndicator(indicator UnreadIndicator) Option {
	return func(service *Service) {
		service.unreadIndicator = indicator
	}
}

// WithExternalPageOpener 注入原生端外部页面窗口能力。
func WithExternalPageOpener(opener ExternalPageOpener) Option {
	return func(service *Service) {
		service.externalPageOpener = opener
	}
}

// New 创建跨平台应用服务。
func New(backend Backend, options ...Option) *Service {
	service := &Service{backend: backend}
	for _, option := range options {
		option(service)
	}
	return service
}

// InstallWorkspace 创建企业管理员并返回登录令牌。
func (s *Service) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	installer, ok := s.backend.(WorkspaceInstaller)
	if !ok {
		return Auth{}, methodNotAllowedError(meta, "InstallWorkspace")
	}
	return installer.InstallWorkspace(ctx, meta, input)
}

// Login 校验账号密码并建立登录会话。
func (s *Service) Login(ctx context.Context, meta RequestMeta, input LoginInput) (Auth, error) {
	auth, err := s.backend.Login(ctx, meta, input)
	if err != nil {
		return Auth{}, err
	}
	s.setNativeLocale(auth.Identity.User.Locale)
	return auth, nil
}

// LoadIdentity 返回当前登录身份。
func (s *Service) LoadIdentity(ctx context.Context, meta RequestMeta) (Identity, error) {
	identity, err := s.backend.LoadIdentity(ctx, meta)
	if err != nil {
		return Identity{}, err
	}
	s.setNativeLocale(identity.User.Locale)
	return identity, nil
}

// UpdateUserPreferences 保存当前用户的偏好设置。
func (s *Service) UpdateUserPreferences(ctx context.Context, meta RequestMeta, input UserPreferencesInput) (CurrentUser, error) {
	user, err := s.backend.UpdateUserPreferences(ctx, meta, input)
	if err != nil {
		return CurrentUser{}, err
	}
	s.setNativeLocale(user.Locale)
	return user, nil
}

// setNativeLocale 在当前平台支持时同步原生界面语言。
func (s *Service) setNativeLocale(locale Locale) {
	if s.nativeLocaleUpdater != nil {
		s.nativeLocaleUpdater.SetLocale(locale)
	}
}

// SelectImage 在原生端选择并读取图片。
func (s *Service) SelectImage(ctx context.Context, meta RequestMeta) (ImageFile, error) {
	if s.imageSelector == nil {
		return ImageFile{}, methodNotAllowedError(meta, "SelectImage")
	}
	return s.imageSelector.SelectImage(ctx, meta)
}

// OpenExternalPage 在原生端应用内新窗口打开外部页面。
func (s *Service) OpenExternalPage(ctx context.Context, meta RequestMeta, input ExternalPageInput) error {
	if s.externalPageOpener == nil {
		return methodNotAllowedError(meta, "OpenExternalPage")
	}
	input.Title = strings.TrimSpace(input.Title)
	input.URL = strings.TrimSpace(input.URL)
	if len(input.URL) > maxExternalPageURLBytes || !common.ValidHTTPURL(input.URL) {
		return InvalidError(meta, cervii18n.ErrorExternalPageURLInvalid, nil)
	}
	return s.externalPageOpener.OpenExternalPage(ctx, meta, input)
}

// maxExternalPageURLBytes 是外部页面地址的最大字节数。
const maxExternalPageURLBytes = 2048

// CheckNotificationPermission 返回当前设备的系统通知授权状态。
func (s *Service) CheckNotificationPermission(ctx context.Context, meta RequestMeta) (NotificationPermissionStatus, error) {
	if s.nativeNotification == nil {
		return NotificationPermissionStatusUnsupported, nil
	}
	return s.nativeNotification.CheckNotificationPermission(ctx, meta)
}

// RequestNotificationPermission 请求当前设备允许发送系统通知。
func (s *Service) RequestNotificationPermission(ctx context.Context, meta RequestMeta) (NotificationPermissionStatus, error) {
	if s.nativeNotification == nil {
		return NotificationPermissionStatusUnsupported, nil
	}
	return s.nativeNotification.RequestNotificationPermission(ctx, meta)
}

// SendMessageNotification 在当前设备投递一条新消息系统通知。
func (s *Service) SendMessageNotification(ctx context.Context, meta RequestMeta, input MessageNotificationInput) error {
	if s.nativeNotification == nil {
		return methodNotAllowedError(meta, "SendMessageNotification")
	}
	return s.nativeNotification.SendMessageNotification(ctx, meta, input)
}

// UpdateUnreadIndicator 更新当前设备的未读提示。
func (s *Service) UpdateUnreadIndicator(_ context.Context, meta RequestMeta, state UnreadIndicatorState) error {
	if s.unreadIndicator == nil {
		return methodNotAllowedError(meta, "UpdateUnreadIndicator")
	}
	return s.unreadIndicator.SetUnreadState(state)
}

// ServerURL 返回原生端当前配置的企业服务器地址。
func (s *Service) ServerURL(ctx context.Context, meta RequestMeta) (string, error) {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return "", methodNotAllowedError(meta, "ServerURL")
	}
	return connector.ServerURL(ctx, meta)
}

// ProbeServer 检测企业服务器并返回公开企业名称，不保存地址。
func (s *Service) ProbeServer(ctx context.Context, meta RequestMeta, serverURL string) (InstallationStatus, error) {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return InstallationStatus{}, methodNotAllowedError(meta, "ProbeServer")
	}
	return connector.ProbeServer(ctx, meta, serverURL)
}

// ConnectServer 验证并保存原生端企业服务器地址。
func (s *Service) ConnectServer(ctx context.Context, meta RequestMeta, serverURL string) error {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return methodNotAllowedError(meta, "ConnectServer")
	}
	return connector.ConnectServer(ctx, meta, serverURL)
}
