//go:build !server

package apiproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
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

// TestProxyRequiresEnterpriseServer 验证未配置企业服务器时拒绝代理请求。
func TestProxyRequiresEnterpriseServer(t *testing.T) {
	service, err := NewService(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/inbox", nil)
	response := httptest.NewRecorder()
	service.ServeHTTP(response, request)

	if response.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusPreconditionRequired)
	}
	assertProxyErrorCode(t, response, "SERVER_CONNECTION_REQUIRED")
}

// TestProxyErrorUsesRequestedLanguage 验证 API 代理本地错误使用请求语言。
func TestProxyErrorUsesRequestedLanguage(t *testing.T) {
	service, err := NewService(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		language string
		message  string
	}{
		{language: "zh-CN", message: "请先连接企业服务器。"},
		{language: "en-US", message: "Connect to your company server first."},
	}

	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, "/inbox", nil)
		request.Header.Set("Accept-Language", test.language)
		response := httptest.NewRecorder()
		service.ServeHTTP(response, request)

		var payload errorBody
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Error.Message != test.message {
			t.Fatalf("message = %q, want %q", payload.Error.Message, test.message)
		}
		if response.Header().Get("Content-Language") != test.language {
			t.Fatalf("Content-Language = %q, want %q", response.Header().Get("Content-Language"), test.language)
		}
	}
}

// TestProxyConnectsAndKeepsRemoteSession 验证 API 代理连接并保持远程会话。
func TestProxyConnectsAndKeepsRemoteSession(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/inbox":
			if cookie, err := request.Cookie("cervi_session"); err == nil && cookie.Value == "session-token" {
				writeJSON(writer, http.StatusOK, map[string]any{"conversations": []any{}})
				return
			}
			writeError(writer, request, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
		case "/api/auth/login":
			http.SetCookie(writer, &http.Cookie{Name: "cervi_session", Value: "session-token", Path: "/", HttpOnly: true})
			writeJSON(writer, http.StatusOK, map[string]any{"user": map[string]string{"email": "owner@example.com"}})
		case "/api/auth/logout":
			http.SetCookie(writer, &http.Cookie{Name: "cervi_session", Value: "", Path: "/", MaxAge: -1})
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer remote.Close()

	store := &memoryStore{}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	configureBody, _ := json.Marshal(map[string]string{"serverUrl": remote.URL})
	configureRequest := httptest.NewRequest(http.MethodPost, serverConnectionPath, bytes.NewReader(configureBody))
	configureRequest.Header.Set("Content-Type", "application/json")
	configureResponse := httptest.NewRecorder()
	service.ServeHTTP(configureResponse, configureRequest)
	if configureResponse.Code != http.StatusNoContent {
		t.Fatalf("configure status = %d, want %d; body=%s", configureResponse.Code, http.StatusNoContent, configureResponse.Body.String())
	}
	if store.serverURL != remote.URL {
		t.Fatalf("saved server URL = %q, want %q", store.serverURL, remote.URL)
	}

	loginRequest := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"owner@example.com","password":"password123"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	service.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.Code, http.StatusOK)
	}

	inboxResponse := httptest.NewRecorder()
	service.ServeHTTP(inboxResponse, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	if inboxResponse.Code != http.StatusOK {
		t.Fatalf("inbox status = %d, want %d; body=%s", inboxResponse.Code, http.StatusOK, inboxResponse.Body.String())
	}

	logoutResponse := httptest.NewRecorder()
	service.ServeHTTP(logoutResponse, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logoutResponse.Code, http.StatusNoContent)
	}

	loggedOutInbox := httptest.NewRecorder()
	service.ServeHTTP(loggedOutInbox, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	if loggedOutInbox.Code != http.StatusUnauthorized {
		t.Fatalf("inbox after logout status = %d, want %d", loggedOutInbox.Code, http.StatusUnauthorized)
	}
}

// TestProbeServerRequiresCerviErrorResponse 验证服务探测拒绝普通 HTTP 接口。
func TestProbeServerRequiresCerviErrorResponse(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	}))
	defer remote.Close()

	baseURL, err := parseServerURL(remote.URL)
	if err != nil {
		t.Fatal(err)
	}
	state, err := newRemoteState(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := probeServer(context.Background(), state); err == nil {
		t.Fatal("expected a non-Cervi HTTP endpoint to be rejected")
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

// assertProxyErrorCode 校验 API 代理返回的业务错误码。
func assertProxyErrorCode(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var payload errorBody
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != expected {
		t.Fatalf("error code = %q, want %q", payload.Error.Code, expected)
	}
}
