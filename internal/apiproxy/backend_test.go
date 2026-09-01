//go:build !server

package apiproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
)

type memoryStore struct {
	serverURL     string
	credential    clientsession.Credential
	credentialSet bool
}

// GetServerURL 返回内存中保存的企业服务器地址。
func (s *memoryStore) GetServerURL(context.Context) (string, error) {
	return s.serverURL, nil
}

// SetServerURL 在内存中保存企业服务器地址。
func (s *memoryStore) SetServerURL(_ context.Context, serverURL string) error {
	s.serverURL = serverURL
	return nil
}

// LoadClientSession 返回内存中保存的原生端登录凭据。
func (s *memoryStore) LoadClientSession(context.Context) (clientsession.Credential, bool, error) {
	return s.credential, s.credentialSet, nil
}

// SaveClientSession 在内存中保存原生端登录凭据。
func (s *memoryStore) SaveClientSession(_ context.Context, credential clientsession.Credential) error {
	s.credential = credential
	s.credentialSet = true
	return nil
}

// DeleteClientSession 删除内存中的原生端登录凭据。
func (s *memoryStore) DeleteClientSession(context.Context) error {
	s.credential = clientsession.Credential{}
	s.credentialSet = false
	return nil
}

// newTestBackend 创建使用内存连接和会话存储的 API Proxy。
func newTestBackend(store *memoryStore) (*Backend, error) {
	sessions, err := clientsession.NewManager(context.Background(), store)
	if err != nil {
		return nil, err
	}
	return NewBackend(store, sessions)
}

// TestBackendRequiresEnterpriseServer 验证未配置企业服务器时拒绝远程调用。
func TestBackendRequiresEnterpriseServer(t *testing.T) {
	backend, err := newTestBackend(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.LoadInbox(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, appservice.LoadInboxInput{})
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.State != appservice.SessionStateConnect {
		t.Fatalf("error = %#v, want connect session", err)
	}
}

// TestBackendPreservesCancellation 验证远程请求取消不会转换为连接错误。
func TestBackendPreservesCancellation(t *testing.T) {
	backend, err := newTestBackend(&memoryStore{serverURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = backend.LoadInbox(ctx, appservice.RequestMeta{}, appservice.LoadInboxInput{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

// TestBackendUnavailablePreservesConnection 验证企业服务器不可用时保留已有连接配置。
func TestBackendUnavailablePreservesConnection(t *testing.T) {
	backend, err := newTestBackend(&memoryStore{serverURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.LoadInbox(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, appservice.LoadInboxInput{})
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.Kind != appservice.ErrorKindUnavailable || apiError.State != "" {
		t.Fatalf("error = %#v, want unavailable without session state", err)
	}
}

// TestBackendConnectsAndUsesBearerToken 验证类型化远程调用使用 Bearer Token。
func TestBackendConnectsAndUsesBearerToken(t *testing.T) {
	const contactAvatarURL = "/storage/organizations/organization-1/files/019d4e1c-40a5-77dd-82e6-6951f9957ba5.png"
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch strings.TrimPrefix(request.URL.Path, "/company") {
		case "/api/installation/status":
			if request.Header.Get("Authorization") != "" {
				http.Error(writer, "installation status must not use login state", http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, map[string]any{"installed": true, "organizationName": "鹿行"})
		case "/api/inbox":
			if request.Header.Get("Authorization") == "Bearer test-token" {
				writeTestJSON(writer, http.StatusOK, map[string]any{
					"conversations": []map[string]any{{
						"id": "conversation-1", "type": "customer", "direct": nil,
						"customer": map[string]any{
							"title": "Telegram 会话", "contactName": "访客",
							"contactAvatarUrl": contactAvatarURL,
							"channelType":      "telegram", "channelName": "Telegram", "preview": "你好",
							"lastMessageAt": time.Now(), "serviceSessionStatus": "open",
						},
					}},
				})
				return
			}
			writeTestJSON(writer, http.StatusUnauthorized, map[string]any{"error": map[string]string{
				"state": "login", "message": "Authentication required.",
			}})
		case "/api/auth/login":
			writeTestJSON(writer, http.StatusOK, map[string]any{
				"identity": map[string]any{
					"organization": map[string]string{"id": "organization-1", "name": "鹿行"},
					"user":         map[string]string{"id": "user-1", "organizationId": "organization-1", "email": "admin@example.com"},
				},
				"token": "test-token", "expiresAt": time.Now().Add(time.Hour),
			})
		case "/api/auth/logout":
			if request.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(writer, "unexpected logout request", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer remote.Close()
	serverURL := remote.URL + "/company"

	store := &memoryStore{}
	backend, err := newTestBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	meta := appservice.RequestMeta{Locale: "zh-CN"}
	status, err := backend.ProbeServer(context.Background(), meta, serverURL)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.OrganizationName != "鹿行" {
		t.Fatalf("probe status = %#v", status)
	}
	if store.serverURL != "" {
		t.Fatalf("probe should not save server URL, got %q", store.serverURL)
	}
	if err := backend.ConnectServer(context.Background(), meta, serverURL); err != nil {
		t.Fatal(err)
	}
	if store.serverURL != serverURL {
		t.Fatalf("server URL = %q, want %q", store.serverURL, serverURL)
	}
	if err := backend.ConnectServer(context.Background(), meta, serverURL); err != nil {
		t.Fatal(err)
	}
	status, err = backend.InstallationStatus(context.Background(), meta)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.OrganizationName != "鹿行" {
		t.Fatalf("status = %#v", status)
	}
	configuredServerURL, err := backend.ServerURL(context.Background(), meta)
	if err != nil {
		t.Fatal(err)
	}
	if configuredServerURL != serverURL {
		t.Fatalf("configured server URL = %q, want %q", configuredServerURL, serverURL)
	}
	auth, err := backend.Login(context.Background(), meta, appservice.LoginInput{Email: "admin@example.com", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	if auth.Token != "" || auth.Identity.User.ID != "user-1" {
		t.Fatalf("native auth = %#v", auth)
	}
	status, err = backend.InstallationStatus(context.Background(), meta)
	if err != nil || !status.Installed {
		t.Fatalf("authenticated installation status = %#v, err = %v", status, err)
	}
	if !store.credentialSet || store.credential.Token != "test-token" || store.credential.UserID != "user-1" || store.credential.OrganizationID != "organization-1" {
		t.Fatalf("saved client credential = %#v, found = %v", store.credential, store.credentialSet)
	}
	if err := backend.ConnectServer(context.Background(), meta, serverURL); err != nil || !store.credentialSet {
		t.Fatalf("same server credential found = %v, error = %v", store.credentialSet, err)
	}
	backend, err = newTestBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := backend.LoadInbox(context.Background(), meta, appservice.LoadInboxInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox.Conversations) != 1 || inbox.Conversations[0].Customer == nil || inbox.Conversations[0].Customer.ContactAvatarURL != serverURL+contactAvatarURL {
		t.Fatalf("normalized inbox = %#v", inbox)
	}
	if err := backend.Logout(context.Background(), meta); err != nil {
		t.Fatal(err)
	}
	if store.credentialSet {
		t.Fatalf("client credential was not deleted: %#v", store.credential)
	}
}

// TestBackendClearsRejectedCredential 验证服务端拒绝认证后删除原生端登录凭据。
func TestBackendClearsRejectedCredential(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/inbox" {
			http.NotFound(writer, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer rejected-token" {
			http.Error(writer, "unexpected authorization", http.StatusBadRequest)
			return
		}
		writeTestJSON(writer, http.StatusUnauthorized, map[string]any{"error": map[string]string{
			"state": "login", "message": "Authentication required.",
		}})
	}))
	defer remote.Close()

	store := &memoryStore{
		serverURL: remote.URL,
		credential: clientsession.Credential{
			ServerURL:      remote.URL,
			OrganizationID: "organization-1",
			UserID:         "user-1",
			Token:          "rejected-token",
			ExpiresAt:      time.Now().Add(time.Hour),
		},
		credentialSet: true,
	}
	backend, err := newTestBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.LoadInbox(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, appservice.LoadInboxInput{})
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.State != appservice.SessionStateLogin {
		t.Fatalf("error = %#v, want login session", err)
	}
	if store.credentialSet {
		t.Fatalf("rejected credential was not deleted: %#v", store.credential)
	}
}

// TestBackendClearsCredentialWhenChangingServer 验证切换企业服务器时删除旧登录凭据。
func TestBackendClearsCredentialWhenChangingServer(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/installation/status" {
			http.NotFound(writer, request)
			return
		}
		writeTestJSON(writer, http.StatusOK, map[string]any{"installed": true, "organizationName": "新企业"})
	}))
	defer remote.Close()

	store := &memoryStore{
		serverURL: "https://old.example.com",
		credential: clientsession.Credential{
			ServerURL:      "https://old.example.com",
			OrganizationID: "organization-1",
			UserID:         "user-1",
			Token:          "old-token",
			ExpiresAt:      time.Now().Add(time.Hour),
		},
		credentialSet: true,
	}
	backend, err := newTestBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.ConnectServer(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, remote.URL); err != nil {
		t.Fatal(err)
	}
	if store.credentialSet {
		t.Fatalf("old credential was not deleted: %#v", store.credential)
	}
}

// TestBackendRejectsUninitializedServer 验证原生端不会保存尚未初始化的企业服务器。
func TestBackendRejectsUninitializedServer(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/installation/status" {
			http.NotFound(writer, request)
			return
		}
		writeTestJSON(writer, http.StatusOK, map[string]bool{"installed": false})
	}))
	defer remote.Close()

	store := &memoryStore{}
	backend, err := newTestBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	err = backend.ConnectServer(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, remote.URL)
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.Kind != appservice.ErrorKindInvalid {
		t.Fatalf("error = %#v, want invalid", err)
	}
	if store.serverURL != "" {
		t.Fatalf("server URL = %q, want empty", store.serverURL)
	}
}

// TestBackendRejectsNonCerviServer 验证原生端拒绝普通 HTTP 服务。
func TestBackendRejectsNonCerviServer(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeTestJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	}))
	defer remote.Close()

	backend, err := newTestBackend(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}
	err = backend.ConnectServer(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, remote.URL)
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.Kind != appservice.ErrorKindUnavailable || apiError.State != "" {
		t.Fatalf("error = %#v, want unavailable without session state", err)
	}
}

// TestParseServerURLRequiresHTTPSOutsideLoopback 验证非回环地址必须使用 HTTPS。
func TestParseServerURLRequiresHTTPSOutsideLoopback(t *testing.T) {
	if _, err := parseServerURL("http://cervi.example.com"); err == nil {
		t.Fatal("expected non-HTTPS remote address to be rejected")
	}
	if _, err := parseServerURL("http://127.0.0.1:8080"); err != nil {
		t.Fatalf("loopback development address was rejected: %v", err)
	}
}

func writeTestJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
