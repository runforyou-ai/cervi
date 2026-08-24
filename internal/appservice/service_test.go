package appservice

import (
	"context"
	"errors"
	"testing"
)

type stubBackend struct {
	Backend
}

type stubNativeLocaleUpdater struct {
	locales []Locale
}

// SetLocale 记录一次原生界面语言同步。
func (u *stubNativeLocaleUpdater) SetLocale(locale Locale) {
	u.locales = append(u.locales, locale)
}

type localeBackend struct {
	Backend
	locale Locale
}

// Login 返回带指定语言的登录身份。
func (b *localeBackend) Login(context.Context, RequestMeta, LoginInput) (Auth, error) {
	return Auth{Identity: Identity{User: CurrentUser{Locale: b.locale}}}, nil
}

// Logout 接受退出登录。
func (*localeBackend) Logout(context.Context, RequestMeta) error {
	return nil
}

// UpdateUserPreferences 返回保存后的用户语言。
func (b *localeBackend) UpdateUserPreferences(context.Context, RequestMeta, UserPreferencesInput) (CurrentUser, error) {
	return CurrentUser{Locale: b.locale}, nil
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

// TestNativeLocaleFollowsAuthenticationAndPreferences 验证原生界面跟随登录和偏好语言。
func TestNativeLocaleFollowsAuthenticationAndPreferences(t *testing.T) {
	backend := &localeBackend{locale: LocaleChineseSimplified}
	updater := &stubNativeLocaleUpdater{}
	service := New(backend, WithNativeLocaleUpdater(updater))

	if _, err := service.Login(context.Background(), RequestMeta{}, LoginInput{}); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	backend.locale = LocaleEnglishUnitedStates
	if _, err := service.UpdateUserPreferences(context.Background(), RequestMeta{}, UserPreferencesInput{}); err != nil {
		t.Fatalf("UpdateUserPreferences() error = %v", err)
	}
	if err := service.Logout(context.Background(), RequestMeta{}); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	want := []Locale{LocaleChineseSimplified, LocaleEnglishUnitedStates}
	if len(updater.locales) != len(want) || updater.locales[0] != want[0] || updater.locales[1] != want[1] {
		t.Fatalf("locales = %v, want %v", updater.locales, want)
	}
}

type sessionBackend struct {
	Backend
	installed   bool
	orgName     string
	identity    Identity
	statusErr   error
	identityErr error
}

func (b *sessionBackend) InstallationStatus(context.Context, RequestMeta) (InstallationStatus, error) {
	if b.statusErr != nil {
		return InstallationStatus{}, b.statusErr
	}
	return InstallationStatus{Installed: b.installed, OrganizationName: b.orgName}, nil
}

func (b *sessionBackend) LoadIdentity(context.Context, RequestMeta) (Identity, error) {
	if b.identityErr != nil {
		return Identity{}, b.identityErr
	}
	return b.identity, nil
}

type nativeSessionBackend struct {
	*sessionBackend
	serverURL string
}

// ServerURL 返回原生端已保存的企业服务器地址。
func (b *nativeSessionBackend) ServerURL(context.Context, RequestMeta) (string, error) {
	return b.serverURL, nil
}

// ProbeServer 返回空的服务器检测结果。
func (b *nativeSessionBackend) ProbeServer(context.Context, RequestMeta, string) (InstallationStatus, error) {
	return InstallationStatus{}, nil
}

// ConnectServer 接受服务器连接。
func (b *nativeSessionBackend) ConnectServer(context.Context, RequestMeta, string) error {
	return nil
}

// TestLoadSessionResolvesWebEntry 验证 Web 端按初始化与登录状态选择入口。
func TestLoadSessionResolvesWebEntry(t *testing.T) {
	identity := Identity{
		Organization: Organization{ID: "organization-1", Name: "鹿行"},
		User:         CurrentUser{ID: "user-1", DisplayName: "管理员"},
	}
	service := New(&sessionBackend{installed: false})
	session, err := service.LoadSession(context.Background(), RequestMeta{})
	if err != nil || session.State != SessionStateSetup {
		t.Fatalf("uninstalled session = %+v, err = %v", session, err)
	}

	service = New(&sessionBackend{installed: true, orgName: "鹿行"})
	session, err = service.LoadSession(context.Background(), RequestMeta{})
	if err != nil || session.State != SessionStateLogin || session.OrganizationName != "鹿行" {
		t.Fatalf("anonymous session = %+v, err = %v", session, err)
	}

	service = New(&sessionBackend{installed: true, orgName: "鹿行", identity: identity})
	session, err = service.LoadSession(context.Background(), RequestMeta{Token: "token"})
	if err != nil || session.State != SessionStateReady || session.Identity == nil || session.Identity.User.ID != "user-1" {
		t.Fatalf("ready session = %+v, err = %v", session, err)
	}

	service = New(&sessionBackend{
		installed:   true,
		orgName:     "鹿行",
		identityErr: &Error{State: SessionStateLogin, Message: "请先登录。"},
	})
	session, err = service.LoadSession(context.Background(), RequestMeta{Token: "expired"})
	if err != nil || session.State != SessionStateLogin || session.OrganizationName != "鹿行" {
		t.Fatalf("expired session = %+v, err = %v", session, err)
	}
}

// TestLoadSessionResolvesNativeEntryWithoutRequestToken 验证原生端由 Backend 解析持久化登录态。
func TestLoadSessionResolvesNativeEntryWithoutRequestToken(t *testing.T) {
	identity := Identity{
		Organization: Organization{ID: "organization-1", Name: "鹿行"},
		User:         CurrentUser{ID: "user-1", DisplayName: "管理员"},
	}
	backend := &nativeSessionBackend{
		sessionBackend: &sessionBackend{installed: true, orgName: "鹿行", identity: identity},
		serverURL:      "https://cervi.example.com",
	}
	session, err := New(backend).LoadSession(context.Background(), RequestMeta{})
	if err != nil || session.State != SessionStateReady || session.Identity == nil || session.Identity.User.ID != "user-1" {
		t.Fatalf("ready native session = %+v, err = %v", session, err)
	}

	backend.identityErr = &Error{State: SessionStateLogin, Message: "请先登录。"}
	session, err = New(backend).LoadSession(context.Background(), RequestMeta{})
	if err != nil || session.State != SessionStateLogin || session.OrganizationName != "鹿行" {
		t.Fatalf("anonymous native session = %+v, err = %v", session, err)
	}
}

// TestLoadSessionPreservesUnavailableNativeConnection 验证原生端服务器暂不可用时不进入连接页。
func TestLoadSessionPreservesUnavailableNativeConnection(t *testing.T) {
	backend := &nativeSessionBackend{
		sessionBackend: &sessionBackend{
			statusErr: &Error{Kind: ErrorKindUnavailable, Message: "暂时无法连接服务器。"},
		},
		serverURL: "https://cervi.example.com",
	}

	session, err := New(backend).LoadSession(context.Background(), RequestMeta{Token: "token"})
	var apiError *Error
	if !errors.As(err, &apiError) || apiError.Kind != ErrorKindUnavailable || session.State != "" {
		t.Fatalf("session = %+v, error = %#v, want unavailable error", session, err)
	}
}

func assertMethodNotAllowed(t *testing.T, err error) {
	t.Helper()
	var apiError *Error
	if !errors.As(err, &apiError) || apiError.Kind != ErrorKindFailed {
		t.Fatalf("error = %#v, want METHOD_NOT_ALLOWED", err)
	}
}
