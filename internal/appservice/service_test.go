package appservice

import (
	"context"
	"errors"
	"testing"
)

type stubBackend struct {
	Backend
}

// TestPlatformMethodsRequireCapability 验证未实现平台能力的 Backend 返回方法不允许。
func TestPlatformMethodsRequireCapability(t *testing.T) {
	service := New(&stubBackend{})
	meta := RequestMeta{Locale: LocaleChineseSimplified}

	_, err := service.InstallWorkspace(context.Background(), meta, InstallWorkspaceInput{})
	assertMethodNotAllowed(t, err)

	_, err = service.ServerURL(context.Background(), meta)
	assertMethodNotAllowed(t, err)

	_, err = service.ProbeServer(context.Background(), meta, "https://cervi.example.com")
	assertMethodNotAllowed(t, err)

	err = service.ConnectServer(context.Background(), meta, "https://cervi.example.com")
	assertMethodNotAllowed(t, err)

	_, err = service.SelectProfileImage(context.Background(), meta)
	assertMethodNotAllowed(t, err)
}

type startupBackend struct {
	Backend
	installed     bool
	orgName       string
	statusErr     error
	identityCalls int
}

// InstallationStatus 返回测试指定的企业初始化状态。
func (b *startupBackend) InstallationStatus(context.Context, RequestMeta) (InstallationStatus, error) {
	if b.statusErr != nil {
		return InstallationStatus{}, b.statusErr
	}
	return InstallationStatus{Installed: b.installed, OrganizationName: b.orgName}, nil
}

// LoadIdentity 返回空身份并累计调用次数。
func (b *startupBackend) LoadIdentity(context.Context, RequestMeta) (Identity, error) {
	b.identityCalls++
	return Identity{}, nil
}

type nativeStartupBackend struct {
	*startupBackend
	serverURL string
}

// ServerURL 返回原生端已保存的企业服务器地址。
func (b *nativeStartupBackend) ServerURL(context.Context, RequestMeta) (string, error) {
	return b.serverURL, nil
}

// ProbeServer 返回空的服务器检测结果。
func (b *nativeStartupBackend) ProbeServer(context.Context, RequestMeta, string) (InstallationStatus, error) {
	return InstallationStatus{}, nil
}

// ConnectServer 接受服务器连接。
func (b *nativeStartupBackend) ConnectServer(context.Context, RequestMeta, string) error {
	return nil
}

// TestLoadStartupResolvesWebEntry 验证 Web 端只按初始化状态选择启动入口。
func TestLoadStartupResolvesWebEntry(t *testing.T) {
	backend := &startupBackend{installed: false}
	service := New(backend)
	startup, err := service.LoadStartup(context.Background(), RequestMeta{})
	if err != nil || startup.State != SessionStateSetup {
		t.Fatalf("uninstalled startup = %+v, err = %v", startup, err)
	}

	backend = &startupBackend{installed: true, orgName: "鹿行"}
	service = New(backend)
	startup, err = service.LoadStartup(context.Background(), RequestMeta{Token: "ignored"})
	if err != nil || startup.State != SessionStateReady || startup.OrganizationName != "鹿行" {
		t.Fatalf("ready startup = %+v, err = %v", startup, err)
	}
	if calls := backend.identityCalls; calls != 0 {
		t.Fatalf("web startup identity calls = %d, want 0", calls)
	}
}

// TestLoadStartupResolvesNativeEntry 验证原生端只检测服务器地址和初始化状态。
func TestLoadStartupResolvesNativeEntry(t *testing.T) {
	backend := &nativeStartupBackend{
		startupBackend: &startupBackend{installed: true, orgName: "鹿行"},
		serverURL:      "https://cervi.example.com",
	}
	startup, err := New(backend).LoadStartup(context.Background(), RequestMeta{})
	if err != nil || startup.State != SessionStateReady || startup.OrganizationName != "鹿行" {
		t.Fatalf("ready native startup = %+v, err = %v", startup, err)
	}

	backend.serverURL = ""
	startup, err = New(backend).LoadStartup(context.Background(), RequestMeta{})
	if err != nil || startup.State != SessionStateConnect {
		t.Fatalf("unconnected native startup = %+v, err = %v", startup, err)
	}

	backend.serverURL = "https://cervi.example.com"
	backend.installed = false
	startup, err = New(backend).LoadStartup(context.Background(), RequestMeta{})
	if err != nil || startup.State != SessionStateConnect {
		t.Fatalf("uninitialized native startup = %+v, err = %v", startup, err)
	}
	if backend.identityCalls != 0 {
		t.Fatalf("native startup identity calls = %d, want 0", backend.identityCalls)
	}
}

// TestLoadStartupRoutesUnavailableNativeServerToConnect 验证原生端服务器不可用时进入连接页。
func TestLoadStartupRoutesUnavailableNativeServerToConnect(t *testing.T) {
	backend := &nativeStartupBackend{
		startupBackend: &startupBackend{
			statusErr: &Error{Kind: ErrorKindUnavailable, Message: "暂时无法连接服务器。"},
		},
		serverURL: "https://cervi.example.com",
	}

	startup, err := New(backend).LoadStartup(context.Background(), RequestMeta{})
	if err != nil || startup.State != SessionStateConnect {
		t.Fatalf("unavailable native startup = %+v, err = %v", startup, err)
	}
	if backend.identityCalls != 0 {
		t.Fatalf("unavailable native startup identity calls = %d, want 0", backend.identityCalls)
	}
}

func assertMethodNotAllowed(t *testing.T, err error) {
	t.Helper()
	var apiError *Error
	if !errors.As(err, &apiError) || apiError.Kind != ErrorKindFailed {
		t.Fatalf("error = %#v, want METHOD_NOT_ALLOWED", err)
	}
}
