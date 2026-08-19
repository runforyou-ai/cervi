//go:build server

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

type testBackend struct {
	appservice.Backend
	lastMeta     appservice.RequestMeta
	lastUserList appservice.UserListInput
}

func (b *testBackend) InstallationStatus(context.Context, appservice.RequestMeta) (bool, error) {
	return true, nil
}

func (b *testBackend) Login(_ context.Context, meta appservice.RequestMeta, input appservice.LoginInput) (appservice.Session, error) {
	b.lastMeta = meta
	if input.Email != "owner@example.com" || input.Password != "password123" {
		return appservice.Session{}, &appservice.Error{Status: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "账号或密码错误。"}
	}
	return appservice.Session{
		Principal: testPrincipal(),
		Token:     "session-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (b *testBackend) LoadSession(_ context.Context, meta appservice.RequestMeta) (appservice.Principal, error) {
	b.lastMeta = meta
	if meta.Token != "session-token" {
		return appservice.Principal{}, &appservice.Error{Status: http.StatusUnauthorized, Code: "AUTH_REQUIRED", Message: "请先登录。"}
	}
	return testPrincipal(), nil
}

func (b *testBackend) ListUsers(_ context.Context, meta appservice.RequestMeta, input appservice.UserListInput) (appservice.UserList, error) {
	b.lastMeta = meta
	b.lastUserList = input
	return appservice.UserList{Users: []appservice.DirectoryUser{}, Page: appservice.PageInfo{Number: input.Page, Size: input.PageSize}}, nil
}

// TestAuthenticationUsesBearerToken 验证登录返回令牌且后续请求读取 Bearer Token。
func TestAuthenticationUsesBearerToken(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	loginResponse := doJSON(t, http.MethodPost, server.URL+"/auth/login", map[string]string{
		"email": "owner@example.com", "password": "password123",
	}, "")
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.StatusCode, http.StatusOK)
	}
	var session appservice.Session
	if err := json.NewDecoder(loginResponse.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.Token != "session-token" {
		t.Fatalf("token = %q, want session-token", session.Token)
	}

	unauthorized := doJSON(t, http.MethodGet, server.URL+"/auth/session", nil, "")
	assertErrorCode(t, unauthorized, http.StatusUnauthorized, "AUTH_REQUIRED")

	authorized := doJSON(t, http.MethodGet, server.URL+"/auth/session", nil, session.Token)
	defer authorized.Body.Close()
	if authorized.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want %d", authorized.StatusCode, http.StatusOK)
	}
	if backend.lastMeta.Token != session.Token {
		t.Fatalf("backend token = %q, want %q", backend.lastMeta.Token, session.Token)
	}
}

// TestListQueryIsConvertedToTypedInput 验证 HTTP 查询参数转换为类型化服务输入。
func TestListQueryIsConvertedToTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodGet, server.URL+"/users?q=lin&status=active&page=2&pageSize=25", nil, "session-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if backend.lastUserList.Query != "lin" || backend.lastUserList.Status != "active" || backend.lastUserList.Page != 2 || backend.lastUserList.PageSize != 25 {
		t.Fatalf("typed input = %#v", backend.lastUserList)
	}
}

// TestInvalidJSONUsesRequestedLanguage 验证 HTTP 适配层的输入错误使用请求语言。
func TestInvalidJSONUsesRequestedLanguage(t *testing.T) {
	server := httptest.NewServer(NewService(appservice.New(&testBackend{})))
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/auth/login", bytes.NewBufferString("{"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "zh-CN")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	assertErrorResponse(t, response, http.StatusBadRequest, "VALIDATION_FAILED")
	if response.Header.Get("Content-Language") != "zh-CN" {
		t.Fatalf("language = %q, want zh-CN", response.Header.Get("Content-Language"))
	}
}

// TestBearerTokenParsing 验证 Bearer Token 请求头解析规则。
func TestBearerTokenParsing(t *testing.T) {
	if token := bearerToken("Bearer session-token"); token != "session-token" {
		t.Fatalf("token = %q, want session-token", token)
	}
	if token := bearerToken("Basic session-token"); token != "" {
		t.Fatalf("basic token = %q, want empty", token)
	}
}

func testPrincipal() appservice.Principal {
	return appservice.Principal{
		Organization: appservice.Organization{ID: "organization-1", Name: "鹿行"},
		User:         appservice.User{ID: "user-1", OrganizationID: "organization-1", Email: "owner@example.com", DisplayName: "所有者", Role: "owner", Status: "active"},
	}
}

func doJSON(t *testing.T, method, endpoint string, body any, token string) *http.Response {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, endpoint, &payload)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func assertErrorCode(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	defer response.Body.Close()
	assertErrorResponse(t, response, status, code)
}

func assertErrorResponse(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	if response.StatusCode != status {
		t.Fatalf("status = %d, want %d", response.StatusCode, status)
	}
	var payload errorBody
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != code {
		t.Fatalf("error code = %q, want %q", payload.Error.Code, code)
	}
}
