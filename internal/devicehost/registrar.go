//go:build !server && !ios && !android

// Package devicehost 把桌面端本机设备注册到当前登录的企业服务器。
package devicehost

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

const (
	// idleInterval 是无事可做时重新检查登录状态的间隔。
	idleInterval = 5 * time.Minute
	// initialRetryInterval 是注册失败后首次重试的间隔，连续失败按倍数退避到 idleInterval。
	initialRetryInterval = 5 * time.Second
	// registerTimeout 是单次注册请求的时限。
	registerTimeout = 30 * time.Second
	// maxNameRunes 是上报设备名称的最大字符数，与服务端校验上限一致；超长主机名截断后上报。
	maxNameRunes = 100
)

// Store 持久化本机安装标识与各企业服务器上的设备注册结果。
type Store interface {
	// DeviceInstallID 读取本机安装标识，尚未生成时创建并保存。
	DeviceInstallID(ctx context.Context) (string, error)
	// LoadDeviceRegistration 读取本机在指定企业服务器上为指定用户注册的设备编号。
	LoadDeviceRegistration(ctx context.Context, serverURL, organizationID, userID string) (string, bool, error)
	// SaveDeviceRegistration 保存本机在指定企业服务器上为指定用户注册的设备编号。
	SaveDeviceRegistration(ctx context.Context, serverURL, organizationID, userID, deviceID string) error
}

// Client 是设备注册使用的企业服务端调用。
type Client interface {
	// ServerURL 返回当前配置的企业服务器地址。
	ServerURL(context.Context, appservice.RequestMeta) (string, error)
	// RegisterDevice 注册当前用户的本机设备。
	RegisterDevice(context.Context, appservice.RequestMeta, appservice.DeviceRegistrationInput) (appservice.Device, error)
}

// Registrar 在登录会话建立后把本机设备注册到企业服务器，每个登录会话注册一次。
type Registrar struct {
	store    Store
	client   Client
	sessions *clientsession.Manager
	name     string
	platform domain.DevicePlatform

	ctx    context.Context
	cancel context.CancelFunc
	wake   chan struct{}
	done   chan struct{}

	mu sync.Mutex
	// registered 是已完成注册的登录会话标识，登录会话变化后重新注册。
	registered string
}

// New 创建桌面端设备注册器；当前平台不支持本机设备时返回 nil。
func New(store Store, client Client, sessions *clientsession.Manager) *Registrar {
	platform, ok := currentPlatform()
	if !ok {
		slog.Info("当前平台不注册本机设备", "os", runtime.GOOS)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Registrar{
		store:    store,
		client:   client,
		sessions: sessions,
		name:     deviceName(),
		platform: platform,
		ctx:      ctx,
		cancel:   cancel,
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
}

// Start 订阅登录凭据变化并开始注册循环。
func (r *Registrar) Start() {
	if r == nil {
		return
	}
	r.sessions.Subscribe(r.Wake)
	go r.run()
}

// Stop 取消进行中的注册，结束注册循环并等待其退出。
func (r *Registrar) Stop() {
	if r == nil {
		return
	}
	r.cancel()
	<-r.done
}

// Wake 请求立即尝试一次注册，循环正在运行时保留一次待处理信号。
func (r *Registrar) Wake() {
	if r == nil {
		return
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// CurrentDevice 返回本机在当前企业服务器上为当前登录用户注册的设备状态。
func (r *Registrar) CurrentDevice(ctx context.Context, meta appservice.RequestMeta) (appservice.LocalDevice, error) {
	if r == nil {
		return appservice.LocalDevice{}, nil
	}
	session, found, err := r.currentDeviceSession(ctx, meta)
	if err != nil || !found {
		return appservice.LocalDevice{}, err
	}
	return appservice.LocalDevice{DeviceID: session.deviceID}, nil
}

// deviceSession 是当前登录会话及本机在该企业服务器上为该用户注册的设备。
type deviceSession struct {
	serverURL  string
	credential clientsession.Credential
	deviceID   string
}

// key 标识一个设备登录会话，换服、换账号、重新登录或重新注册后取值变化。
func (s deviceSession) key() string {
	return sessionKey(s.serverURL, s.credential) + "\n" + s.deviceID
}

// currentDeviceSession 返回当前登录会话及本机已注册的设备，尚未登录或尚未注册时返回 false。
func (r *Registrar) currentDeviceSession(ctx context.Context, meta appservice.RequestMeta) (deviceSession, bool, error) {
	serverURL, credential, ok := r.currentSession(ctx, meta)
	if !ok {
		return deviceSession{}, false, nil
	}
	deviceID, found, err := r.store.LoadDeviceRegistration(ctx, serverURL, credential.OrganizationID, credential.UserID)
	if err != nil {
		if ctx.Err() != nil {
			return deviceSession{}, false, ctx.Err()
		}
		return deviceSession{}, false, fmt.Errorf("load device registration: %w", err)
	}
	if !found {
		return deviceSession{}, false, nil
	}
	return deviceSession{serverURL: serverURL, credential: credential, deviceID: deviceID}, true, nil
}

// run 在唤醒信号和重试间隔上尝试注册，直到注册器停止；注册失败按退避缩短下次尝试的等待。
func (r *Registrar) run() {
	defer close(r.done)
	backoff := initialRetryInterval
	for {
		wait := idleInterval
		if r.register() {
			backoff = initialRetryInterval
		} else {
			wait = backoff
			backoff = min(backoff*2, idleInterval)
		}
		select {
		case <-r.ctx.Done():
			return
		case <-r.wake:
		case <-time.After(wait):
		}
	}
}

// register 在已登录且当前登录会话尚未注册时上报本机设备、本机运行时版本与本机工具清单，返回本次是否无需尽快重试。
func (r *Registrar) register() bool {
	ctx, cancel := context.WithTimeout(r.ctx, registerTimeout)
	defer cancel()
	meta := appservice.RequestMeta{}
	serverURL, credential, ok := r.currentSession(ctx, meta)
	if !ok {
		return true
	}
	session := sessionKey(serverURL, credential)
	if r.registeredFor(session) {
		return true
	}
	installID, err := r.store.DeviceInstallID(ctx)
	if err != nil {
		slog.Warn("读取本机安装标识失败", "error", err)
		return false
	}
	device, err := r.client.RegisterDevice(ctx, meta, appservice.DeviceRegistrationInput{
		InstallID: installID, Name: r.name, Platform: appservice.DevicePlatform(r.platform),
		RuntimeVersion: agentruntime.LocalRuntimeVersion, ToolManifest: agentruntime.LocalToolManifest(),
	})
	if err != nil {
		slog.Warn("注册本机设备失败", "server_url", serverURL, "organization_id", credential.OrganizationID, "error", err)
		return false
	}
	if err := r.store.SaveDeviceRegistration(ctx, serverURL, credential.OrganizationID, credential.UserID, device.ID); err != nil {
		slog.Warn("保存本机设备注册结果失败", "server_url", serverURL, "organization_id", credential.OrganizationID, "device_id", device.ID, "error", err)
		return false
	}
	r.markRegistered(session)
	slog.Info("本机设备已注册", "server_url", serverURL, "organization_id", credential.OrganizationID, "user_id", credential.UserID, "device_id", device.ID, "name", device.Name)
	return true
}

// currentSession 返回当前企业服务器地址及其有效登录凭据。
func (r *Registrar) currentSession(ctx context.Context, meta appservice.RequestMeta) (string, clientsession.Credential, bool) {
	serverURL, err := r.client.ServerURL(ctx, meta)
	if err != nil || serverURL == "" {
		return "", clientsession.Credential{}, false
	}
	credential, ok := r.sessions.Current(ctx, serverURL)
	if !ok {
		return "", clientsession.Credential{}, false
	}
	return serverURL, credential, true
}

// registeredFor 判断指定登录会话是否已完成注册。
func (r *Registrar) registeredFor(session string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.registered == session
}

// markRegistered 记录指定登录会话已完成注册。
func (r *Registrar) markRegistered(session string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registered = session
}

// sessionKey 标识一个登录会话，换服、换账号或重新登录后取值变化。
func sessionKey(serverURL string, credential clientsession.Credential) string {
	return serverURL + "\n" + credential.OrganizationID + "\n" + credential.UserID + "\n" + credential.Token
}

// currentPlatform 返回当前运行平台对应的设备平台。
func currentPlatform() (domain.DevicePlatform, bool) {
	switch runtime.GOOS {
	case "darwin":
		return domain.DevicePlatformMacOS, true
	case "windows":
		return domain.DevicePlatformWindows, true
	case "linux":
		return domain.DevicePlatformLinux, true
	}
	return "", false
}

// deviceName 返回本机名称，读取失败时使用平台名称，超出服务端上限时截断。
func deviceName() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		slog.Warn("读取本机名称失败", "error", err)
		return runtime.GOOS
	}
	if runes := []rune(hostname); len(runes) > maxNameRunes {
		return string(runes[:maxNameRunes])
	}
	return hostname
}
