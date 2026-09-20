//go:build !server && !ios && !android

package devicehost

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
)

// stubStore 在内存中保存本机安装标识与设备注册结果。
type stubStore struct {
	installID     string
	registrations map[string]string
}

// DeviceInstallID 返回固定的本机安装标识。
func (s *stubStore) DeviceInstallID(context.Context) (string, error) { return s.installID, nil }

// LoadDeviceRegistration 读取内存中的设备编号。
func (s *stubStore) LoadDeviceRegistration(_ context.Context, serverURL, organizationID, userID string) (string, bool, error) {
	deviceID, found := s.registrations[serverURL+"|"+organizationID+"|"+userID]
	return deviceID, found, nil
}

// SaveDeviceRegistration 保存内存中的设备编号。
func (s *stubStore) SaveDeviceRegistration(_ context.Context, serverURL, organizationID, userID, deviceID string) error {
	s.registrations[serverURL+"|"+organizationID+"|"+userID] = deviceID
	return nil
}

// stubClient 记录注册调用并返回预设设备。
type stubClient struct {
	serverURL string
	deviceID  string
	failure   error
	calls     []appservice.DeviceRegistrationInput
}

// ServerURL 返回预设的企业服务器地址。
func (c *stubClient) ServerURL(context.Context, appservice.RequestMeta) (string, error) {
	return c.serverURL, nil
}

// RegisterDevice 记录一次注册调用。
func (c *stubClient) RegisterDevice(_ context.Context, _ appservice.RequestMeta, input appservice.DeviceRegistrationInput) (appservice.Device, error) {
	c.calls = append(c.calls, input)
	if c.failure != nil {
		return appservice.Device{}, c.failure
	}
	return appservice.Device{ID: c.deviceID, Name: input.Name}, nil
}

// stubSessionStore 提供一份可替换的原生端登录凭据。
type stubSessionStore struct {
	credential clientsession.Credential
	found      bool
}

// LoadClientSession 返回当前登录凭据。
func (s *stubSessionStore) LoadClientSession(context.Context) (clientsession.Credential, bool, error) {
	return s.credential, s.found, nil
}

// SaveClientSession 保存当前登录凭据。
func (s *stubSessionStore) SaveClientSession(_ context.Context, credential clientsession.Credential) error {
	s.credential, s.found = credential, true
	return nil
}

// DeleteClientSession 删除当前登录凭据。
func (s *stubSessionStore) DeleteClientSession(context.Context) error {
	s.credential, s.found = clientsession.Credential{}, false
	return nil
}

// newTestRegistrar 创建使用内存依赖的注册器。
func newTestRegistrar(t *testing.T, store *stubStore, client *stubClient) (*Registrar, *clientsession.Manager) {
	t.Helper()
	sessions, err := clientsession.NewManager(context.Background(), &stubSessionStore{})
	if err != nil {
		t.Fatal(err)
	}
	registrar := New(store, client, sessions)
	if registrar == nil {
		t.Skip("当前平台不注册本机设备")
	}
	return registrar, sessions
}

// credentialFor 构造指定企业、用户和令牌的登录凭据。
func credentialFor(serverURL, organizationID, userID, token string) clientsession.Credential {
	return clientsession.Credential{
		ServerURL: serverURL, OrganizationID: organizationID, UserID: userID,
		Token: token, ExpiresAt: time.Now().Add(time.Hour),
	}
}

// TestRegisterSkipsWithoutSession 验证尚未登录时不上报本机设备。
func TestRegisterSkipsWithoutSession(t *testing.T) {
	client := &stubClient{serverURL: "https://cervi.example.com", deviceID: "device-1"}
	registrar, _ := newTestRegistrar(t, &stubStore{installID: "install-1", registrations: map[string]string{}}, client)

	registrar.register()

	if len(client.calls) != 0 {
		t.Fatalf("未登录时的注册次数 = %d", len(client.calls))
	}
}

// TestRegisterOncePerLoginSession 验证同一登录会话只注册一次，重新登录后再次注册。
func TestRegisterOncePerLoginSession(t *testing.T) {
	ctx := context.Background()
	const serverURL = "https://cervi.example.com"
	store := &stubStore{installID: "install-1", registrations: map[string]string{}}
	client := &stubClient{serverURL: serverURL, deviceID: "device-1"}
	registrar, sessions := newTestRegistrar(t, store, client)

	if err := sessions.Establish(ctx, credentialFor(serverURL, "org-1", "user-1", "token-1")); err != nil {
		t.Fatal(err)
	}
	registrar.register()
	registrar.register()
	if len(client.calls) != 1 {
		t.Fatalf("同一登录会话的注册次数 = %d", len(client.calls))
	}
	if client.calls[0].InstallID != "install-1" {
		t.Fatalf("上报的安装标识 = %q", client.calls[0].InstallID)
	}

	if err := sessions.Establish(ctx, credentialFor(serverURL, "org-1", "user-1", "token-2")); err != nil {
		t.Fatal(err)
	}
	registrar.register()
	if len(client.calls) != 2 {
		t.Fatalf("重新登录后的注册次数 = %d", len(client.calls))
	}
}

// TestRegisterRetriesAfterFailure 验证注册失败后下一次尝试继续上报。
func TestRegisterRetriesAfterFailure(t *testing.T) {
	ctx := context.Background()
	const serverURL = "https://cervi.example.com"
	store := &stubStore{installID: "install-1", registrations: map[string]string{}}
	client := &stubClient{serverURL: serverURL, deviceID: "device-1", failure: errors.New("服务器不可达")}
	registrar, sessions := newTestRegistrar(t, store, client)
	if err := sessions.Establish(ctx, credentialFor(serverURL, "org-1", "user-1", "token-1")); err != nil {
		t.Fatal(err)
	}

	registrar.register()
	client.failure = nil
	registrar.register()

	if len(client.calls) != 2 {
		t.Fatalf("注册次数 = %d", len(client.calls))
	}
	device, err := registrar.CurrentDevice(ctx, appservice.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if device.DeviceID != "device-1" {
		t.Fatalf("本机设备编号 = %q", device.DeviceID)
	}
}

// TestCurrentDeviceWithoutRegistration 验证尚未注册时返回空设备编号。
func TestCurrentDeviceWithoutRegistration(t *testing.T) {
	ctx := context.Background()
	const serverURL = "https://cervi.example.com"
	store := &stubStore{installID: "install-1", registrations: map[string]string{}}
	registrar, sessions := newTestRegistrar(t, store, &stubClient{serverURL: serverURL, deviceID: "device-1"})
	if err := sessions.Establish(ctx, credentialFor(serverURL, "org-1", "user-1", "token-1")); err != nil {
		t.Fatal(err)
	}

	device, err := registrar.CurrentDevice(ctx, appservice.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if device.DeviceID != "" {
		t.Fatalf("尚未注册时的设备编号 = %q", device.DeviceID)
	}
}

// TestCurrentDeviceSeparatesAccounts 验证同一台机器切换账号后不返回上一账号的设备编号。
func TestCurrentDeviceSeparatesAccounts(t *testing.T) {
	ctx := context.Background()
	const serverURL = "https://cervi.example.com"
	store := &stubStore{installID: "install-1", registrations: map[string]string{}}
	client := &stubClient{serverURL: serverURL, deviceID: "device-1"}
	registrar, sessions := newTestRegistrar(t, store, client)
	if err := sessions.Establish(ctx, credentialFor(serverURL, "org-1", "user-1", "token-1")); err != nil {
		t.Fatal(err)
	}
	registrar.register()

	// 切换到同一企业的另一个账号，该账号的注册尚未成功。
	client.failure = errors.New("服务器不可达")
	if err := sessions.Establish(ctx, credentialFor(serverURL, "org-1", "user-2", "token-2")); err != nil {
		t.Fatal(err)
	}
	registrar.register()

	device, err := registrar.CurrentDevice(ctx, appservice.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if device.DeviceID != "" {
		t.Fatalf("切换账号后的本机设备编号 = %q", device.DeviceID)
	}
}
