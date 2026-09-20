//go:build !server && !ios && !android

// Package devicehost 把桌面端本机设备注册到当前登录的企业服务器。
package devicehost

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// retryInterval 是尚未登录或注册失败时重新尝试的间隔。
const retryInterval = 5 * time.Minute

// registerTimeout 是单次注册请求的时限。
const registerTimeout = 30 * time.Second

// Store 持久化本机安装标识与各企业服务器上的设备注册结果。
type Store interface {
	// DeviceInstallID 读取本机安装标识，尚未生成时创建并保存。
	DeviceInstallID(ctx context.Context) (string, error)
	// LoadDeviceRegistration 读取本机在指定企业服务器上的设备编号。
	LoadDeviceRegistration(ctx context.Context, serverURL, organizationID string) (string, bool, error)
	// SaveDeviceRegistration 保存本机在指定企业服务器上的设备编号。
	SaveDeviceRegistration(ctx context.Context, serverURL, organizationID, deviceID string) error
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
	version  string

	wake chan struct{}
	stop chan struct{}
	done chan struct{}

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
	return &Registrar{
		store:    store,
		client:   client,
		sessions: sessions,
		name:     deviceName(),
		platform: platform,
		version:  runtimeVersion(),
		wake:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
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

// Stop 结束注册循环并等待其退出。
func (r *Registrar) Stop() {
	if r == nil {
		return
	}
	close(r.stop)
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

// CurrentDevice 返回本机在当前企业服务器上的设备注册状态。
func (r *Registrar) CurrentDevice(ctx context.Context, meta appservice.RequestMeta) (appservice.LocalDevice, error) {
	if r == nil {
		return appservice.LocalDevice{}, nil
	}
	serverURL, credential, ok := r.currentSession(ctx, meta)
	if !ok {
		return appservice.LocalDevice{}, nil
	}
	deviceID, found, err := r.store.LoadDeviceRegistration(ctx, serverURL, credential.OrganizationID)
	if err != nil {
		if ctx.Err() != nil {
			return appservice.LocalDevice{}, ctx.Err()
		}
		return appservice.LocalDevice{}, fmt.Errorf("load device registration: %w", err)
	}
	if !found {
		return appservice.LocalDevice{}, nil
	}
	return appservice.LocalDevice{DeviceID: deviceID}, nil
}

// run 在唤醒信号和重试间隔上尝试注册，直到注册器停止。
func (r *Registrar) run() {
	defer close(r.done)
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	for {
		r.register()
		select {
		case <-r.stop:
			return
		case <-r.wake:
		case <-ticker.C:
		}
	}
}

// register 在已登录且当前登录会话尚未注册时上报本机设备。
func (r *Registrar) register() {
	ctx, cancel := context.WithTimeout(context.Background(), registerTimeout)
	defer cancel()
	meta := appservice.RequestMeta{}
	serverURL, credential, ok := r.currentSession(ctx, meta)
	if !ok {
		return
	}
	session := sessionKey(serverURL, credential)
	if r.registeredFor(session) {
		return
	}
	installID, err := r.store.DeviceInstallID(ctx)
	if err != nil {
		slog.Warn("读取本机安装标识失败", "error", err)
		return
	}
	device, err := r.client.RegisterDevice(ctx, meta, appservice.DeviceRegistrationInput{
		InstallID: installID, Name: r.name, Platform: appservice.DevicePlatform(r.platform), RuntimeVersion: r.version,
	})
	if err != nil {
		slog.Warn("注册本机设备失败", "server_url", serverURL, "organization_id", credential.OrganizationID, "error", err)
		return
	}
	if err := r.store.SaveDeviceRegistration(ctx, serverURL, credential.OrganizationID, device.ID); err != nil {
		slog.Warn("保存本机设备注册结果失败", "server_url", serverURL, "organization_id", credential.OrganizationID, "device_id", device.ID, "error", err)
		return
	}
	r.markRegistered(session)
	slog.Info("本机设备已注册", "server_url", serverURL, "organization_id", credential.OrganizationID, "device_id", device.ID, "name", device.Name)
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
	return serverURL + "\n" + credential.OrganizationID + "\n" + credential.Token
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

// deviceName 返回本机名称，读取失败时使用平台名称。
func deviceName() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		slog.Warn("读取本机名称失败", "error", err)
		return runtime.GOOS
	}
	return hostname
}

// runtimeVersion 返回设备侧运行时版本。
func runtimeVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}
