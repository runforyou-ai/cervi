//go:build !server

package apiproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

type memoryStore struct {
	serverURL string
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

// TestBackendRequiresEnterpriseServer 验证未配置企业服务器时拒绝远程调用。
func TestBackendRequiresEnterpriseServer(t *testing.T) {
	backend, err := NewBackend(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.LoadInbox(context.Background(), appservice.RequestMeta{Locale: "zh-CN"})
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.State != appservice.SessionStateConnect {
		t.Fatalf("error = %#v, want connect session", err)
	}
}

// TestBackendPreservesCancellation 验证远程请求取消不会转换为连接错误。
func TestBackendPreservesCancellation(t *testing.T) {
	backend, err := NewBackend(&memoryStore{serverURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = backend.LoadInbox(ctx, appservice.RequestMeta{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

// TestBackendUnavailablePreservesConnection 验证企业服务器不可用时保留已有连接配置。
func TestBackendUnavailablePreservesConnection(t *testing.T) {
	backend, err := NewBackend(&memoryStore{serverURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.LoadInbox(context.Background(), appservice.RequestMeta{Locale: "zh-CN"})
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.Kind != appservice.ErrorKindUnavailable || apiError.State != "" {
		t.Fatalf("error = %#v, want unavailable without session state", err)
	}
}

// TestBackendConnectsAndUsesBearerToken 验证类型化远程调用使用 Bearer Token。
func TestBackendConnectsAndUsesBearerToken(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/installation/status":
			writeTestJSON(writer, http.StatusOK, map[string]any{"installed": true, "organizationName": "鹿行"})
		case "/api/inbox":
			if request.Header.Get("Authorization") == "Bearer test-token" {
				writeTestJSON(writer, http.StatusOK, map[string]any{
					"organization":  map[string]string{"id": "organization-1", "name": "鹿行"},
					"user":          map[string]string{"id": "user-1", "organizationId": "organization-1", "email": "owner@example.com"},
					"conversations": []any{},
				})
				return
			}
			writeTestJSON(writer, http.StatusUnauthorized, map[string]any{"error": map[string]string{
				"state": "login", "message": "Authentication required.",
			}})
		case "/api/profile":
			if request.Method != http.MethodPatch || request.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(writer, "unexpected profile request", http.StatusBadRequest)
				return
			}
			var input appservice.ProfileInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, map[string]string{
				"id": "user-1", "organizationId": "organization-1", "displayName": input.DisplayName, "email": input.Email,
			})
		case "/api/password":
			if request.Method != http.MethodPatch || request.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(writer, "unexpected password request", http.StatusBadRequest)
				return
			}
			var input appservice.ChangePasswordInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			if input.CurrentPassword != "password123" || input.NewPassword != "new-password123" {
				http.Error(writer, "unexpected password input", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		case "/api/preferences":
			if request.Method != http.MethodPatch || request.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(writer, "unexpected preferences request", http.StatusBadRequest)
				return
			}
			var input appservice.UserPreferencesInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, map[string]string{
				"id": "user-1", "organizationId": "organization-1", "locale": string(input.Locale), "timeZone": input.TimeZone,
			})
		case "/api/work-status":
			if request.Method != http.MethodPatch || request.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(writer, "unexpected work status request", http.StatusBadRequest)
				return
			}
			var input appservice.UserWorkStatusInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, map[string]string{
				"id": "user-1", "organizationId": "organization-1", "workStatus": string(input.WorkStatus),
			})
		case "/api/auth/login":
			writeTestJSON(writer, http.StatusOK, map[string]any{
				"identity": map[string]any{
					"organization": map[string]string{"id": "organization-1", "name": "鹿行"},
					"user":         map[string]string{"id": "user-1", "organizationId": "organization-1", "email": "owner@example.com"},
				},
				"token": "test-token", "expiresAt": time.Now().Add(time.Hour),
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer remote.Close()

	store := &memoryStore{}
	backend, err := NewBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	meta := appservice.RequestMeta{Locale: "zh-CN"}
	status, err := backend.ProbeServer(context.Background(), meta, remote.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.OrganizationName != "鹿行" {
		t.Fatalf("probe status = %#v", status)
	}
	if store.serverURL != "" {
		t.Fatalf("probe should not save server URL, got %q", store.serverURL)
	}
	changed, err := backend.ConnectServer(context.Background(), meta, remote.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first server connection should be marked as changed")
	}
	if store.serverURL != remote.URL {
		t.Fatalf("server URL = %q, want %q", store.serverURL, remote.URL)
	}
	changed, err = backend.ConnectServer(context.Background(), meta, remote.URL)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("same server connection should not be marked as changed")
	}
	status, err = backend.InstallationStatus(context.Background(), meta)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.OrganizationName != "鹿行" {
		t.Fatalf("status = %#v", status)
	}
	serverURL, err := backend.ServerURL(context.Background(), meta)
	if err != nil {
		t.Fatal(err)
	}
	if serverURL != remote.URL {
		t.Fatalf("configured server URL = %q, want %q", serverURL, remote.URL)
	}
	auth, err := backend.Login(context.Background(), meta, appservice.LoginInput{Email: "owner@example.com", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	if auth.Token != "test-token" {
		t.Fatalf("token = %q, want test-token", auth.Token)
	}
	meta.Token = auth.Token
	if _, err := backend.LoadInbox(context.Background(), meta); err != nil {
		t.Fatal(err)
	}
	user, err := backend.UpdateProfile(context.Background(), meta, appservice.ProfileInput{
		DisplayName: "林晓",
		Email:       "lin@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.DisplayName != "林晓" || user.Email != "lin@example.com" {
		t.Fatalf("updated user = %#v", user)
	}
	if err := backend.ChangePassword(context.Background(), meta, appservice.ChangePasswordInput{
		CurrentPassword: "password123",
		NewPassword:     "new-password123",
	}); err != nil {
		t.Fatal(err)
	}
	preferences, err := backend.UpdateUserPreferences(context.Background(), meta, appservice.UserPreferencesInput{
		Locale: appservice.LocaleEnglishUnitedStates, TimeZone: "America/New_York",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preferences.Locale != appservice.LocaleEnglishUnitedStates || preferences.TimeZone != "America/New_York" {
		t.Fatalf("updated preferences = %#v", preferences)
	}
	workStatus, err := backend.UpdateUserWorkStatus(context.Background(), meta, appservice.UserWorkStatusInput{
		WorkStatus: appservice.WorkStatusAway,
	})
	if err != nil {
		t.Fatal(err)
	}
	if workStatus.WorkStatus != appservice.WorkStatusAway {
		t.Fatalf("updated work status = %#v", workStatus)
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
	backend, err := NewBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.ConnectServer(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, remote.URL)
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

	backend, err := NewBackend(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.ConnectServer(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, remote.URL)
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
