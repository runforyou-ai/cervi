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
	if !errors.As(err, &apiError) || apiError.Code != "SERVER_CONNECTION_REQUIRED" {
		t.Fatalf("error = %#v, want SERVER_CONNECTION_REQUIRED", err)
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

// TestBackendConnectsAndUsesBearerToken 验证类型化远程调用使用 Bearer Token。
func TestBackendConnectsAndUsesBearerToken(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/installation/status":
			writeTestJSON(writer, http.StatusOK, map[string]bool{"installed": true})
		case "/api/inbox":
			if request.Header.Get("Authorization") == "Bearer session-token" {
				writeTestJSON(writer, http.StatusOK, map[string]any{
					"organization":  map[string]string{"id": "organization-1", "name": "鹿行"},
					"user":          map[string]string{"id": "user-1", "organizationId": "organization-1", "email": "owner@example.com"},
					"conversations": []any{},
				})
				return
			}
			writeTestJSON(writer, http.StatusUnauthorized, map[string]any{"error": map[string]string{
				"code": "AUTH_REQUIRED", "message": "Authentication required.",
			}})
		case "/api/auth/login":
			writeTestJSON(writer, http.StatusOK, map[string]any{
				"principal": map[string]any{
					"organization": map[string]string{"id": "organization-1", "name": "鹿行"},
					"user":         map[string]string{"id": "user-1", "organizationId": "organization-1", "email": "owner@example.com"},
				},
				"token": "session-token", "expiresAt": time.Now().Add(time.Hour),
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
	if err := backend.ConnectServer(context.Background(), meta, remote.URL); err != nil {
		t.Fatal(err)
	}
	if store.serverURL != remote.URL {
		t.Fatalf("server URL = %q, want %q", store.serverURL, remote.URL)
	}
	serverURL, err := backend.ServerURL(context.Background(), meta)
	if err != nil {
		t.Fatal(err)
	}
	if serverURL != remote.URL {
		t.Fatalf("configured server URL = %q, want %q", serverURL, remote.URL)
	}
	session, err := backend.Login(context.Background(), meta, appservice.LoginInput{Email: "owner@example.com", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	if session.Token != "session-token" {
		t.Fatalf("token = %q, want session-token", session.Token)
	}
	meta.Token = session.Token
	if _, err := backend.LoadInbox(context.Background(), meta); err != nil {
		t.Fatal(err)
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
	err = backend.ConnectServer(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, remote.URL)
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.Code != "SERVER_INITIALIZATION_REQUIRED" {
		t.Fatalf("error = %#v, want SERVER_INITIALIZATION_REQUIRED", err)
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
	err = backend.ConnectServer(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, remote.URL)
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.Code != "SERVER_UNAVAILABLE" {
		t.Fatalf("error = %#v, want SERVER_UNAVAILABLE", err)
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
