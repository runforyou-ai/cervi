//go:build server

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

type memoryApplication struct {
	installed bool
	principal servermodels.Principal
	password  string
	sessions  map[string]time.Time
	channels  map[string]servermodels.Channel
	nextID    int
	deleteErr error
}

// newMemoryApplication 创建未初始化的内存测试应用。
func newMemoryApplication() *memoryApplication {
	return &memoryApplication{
		sessions: make(map[string]time.Time),
		channels: make(map[string]servermodels.Channel),
	}
}

// newTestService 使用内存执行函数组装测试 API。
func newTestService(application *memoryApplication) *Service {
	return NewService(Dependencies{
		InstallWorkspace:      application.install,
		Login:                 application.login,
		Logout:                application.logout,
		ResolveSession:        application.resolveSession,
		Installation:          application.installationStatus,
		LoadInbox:             application.loadInbox,
		ListWebsiteChannels:   application.listWebsiteChannels,
		GetWebsiteChannel:     application.getWebsiteChannel,
		CreateWebsiteChannel:  application.createWebsiteChannel,
		UpdateWebsiteChannel:  application.updateWebsiteChannel,
		DeleteWebsiteChannel:  application.deleteWebsiteChannel,
		RestoreWebsiteChannel: application.restoreWebsiteChannel,
	})
}

// install 在内存中创建企业所有者和初始会话。
func (a *memoryApplication) install(_ context.Context, input installationaction.InstallWorkspaceInput) (installationaction.InstallWorkspaceOutput, error) {
	if a.installed {
		return installationaction.InstallWorkspaceOutput{}, installationaction.ErrAlreadyInstalled
	}
	a.installed = true
	a.password = input.Password
	a.principal = servermodels.Principal{
		Organization: servermodels.Organization{ID: "organization-1", Name: input.OrganizationName},
		User: servermodels.User{
			ID:             "user-1",
			OrganizationID: "organization-1",
			Email:          strings.ToLower(input.Email),
			DisplayName:    input.DisplayName,
			Role:           "owner",
			Status:         "active",
		},
	}
	expiresAt := time.Now().Add(time.Hour)
	a.sessions["install-token"] = expiresAt
	principal := a.principal
	return installationaction.InstallWorkspaceOutput{
		Principal: &principal,
		Token:     "install-token",
		ExpiresAt: expiresAt,
	}, nil
}

// login 在内存中校验测试账号并创建会话。
func (a *memoryApplication) login(_ context.Context, input authaction.LoginInput) (authaction.LoginOutput, error) {
	if !a.installed || !strings.EqualFold(strings.TrimSpace(input.Email), a.principal.User.Email) || input.Password != a.password {
		return authaction.LoginOutput{}, authaction.ErrInvalidCredentials
	}
	expiresAt := time.Now().Add(time.Hour)
	a.sessions["login-token"] = expiresAt
	principal := a.principal
	return authaction.LoginOutput{Principal: &principal, Token: "login-token", ExpiresAt: expiresAt}, nil
}

// logout 在内存中删除测试会话。
func (a *memoryApplication) logout(_ context.Context, token string) error {
	if a.deleteErr != nil {
		return a.deleteErr
	}
	delete(a.sessions, token)
	return nil
}

// resolveSession 查找有效测试会话对应的用户身份。
func (a *memoryApplication) resolveSession(_ context.Context, token string) (*servermodels.Principal, error) {
	expiresAt, exists := a.sessions[token]
	if !exists || !expiresAt.After(time.Now()) {
		return nil, nil
	}
	principal := a.principal
	return &principal, nil
}

// installationStatus 返回内存测试应用的安装状态。
func (a *memoryApplication) installationStatus(context.Context) (bool, error) {
	return a.installed, nil
}

// loadInbox 返回内存测试用户的空收件箱。
func (a *memoryApplication) loadInbox(_ context.Context, principal *servermodels.Principal) inboxaction.LoadInboxOutput {
	return inboxaction.LoadInboxOutput{
		Organization:  principal.Organization,
		User:          principal.User,
		Conversations: []any{},
	}
}

// listWebsiteChannels 按删除状态返回当前企业的网站渠道。
func (a *memoryApplication) listWebsiteChannels(_ context.Context, principal *servermodels.Principal, deleted bool) ([]servermodels.Channel, error) {
	channels := make([]servermodels.Channel, 0)
	for _, item := range a.channels {
		if item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite {
			continue
		}
		if (item.DeletedAt != nil) != deleted {
			continue
		}
		channels = append(channels, item)
	}
	return channels, nil
}

// getWebsiteChannel 返回当前企业中未删除的网站渠道。
func (a *memoryApplication) getWebsiteChannel(_ context.Context, principal *servermodels.Principal, channelID string) (*servermodels.Channel, error) {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt != nil {
		return nil, channelaction.ErrNotFound
	}
	return &item, nil
}

// createWebsiteChannel 创建内存网站渠道。
func (a *memoryApplication) createWebsiteChannel(_ context.Context, principal *servermodels.Principal, input channelaction.WebsiteChannelInput) (*servermodels.Channel, error) {
	a.nextID++
	now := time.Now()
	item := servermodels.Channel{
		ID:              "channel-" + strconv.Itoa(a.nextID),
		OrganizationID:  principal.Organization.ID,
		CreatedByUserID: principal.User.ID,
		Type:            channelaction.TypeWebsite,
		Name:            strings.TrimSpace(input.Name),
		DefaultLocale:   input.DefaultLocale,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if description := strings.TrimSpace(input.Description); description != "" {
		item.Description = &description
	}
	a.channels[item.ID] = item
	return &item, nil
}

// updateWebsiteChannel 修改内存网站渠道。
func (a *memoryApplication) updateWebsiteChannel(_ context.Context, principal *servermodels.Principal, channelID string, input channelaction.WebsiteChannelInput) (*servermodels.Channel, error) {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt != nil {
		return nil, channelaction.ErrNotFound
	}
	item.Name = strings.TrimSpace(input.Name)
	item.Description = nil
	if description := strings.TrimSpace(input.Description); description != "" {
		item.Description = &description
	}
	item.DefaultLocale = input.DefaultLocale
	item.UpdatedAt = time.Now()
	a.channels[channelID] = item
	return &item, nil
}

// deleteWebsiteChannel 软删除内存网站渠道。
func (a *memoryApplication) deleteWebsiteChannel(_ context.Context, principal *servermodels.Principal, channelID string) error {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt != nil {
		return channelaction.ErrNotFound
	}
	now := time.Now()
	item.DeletedAt = &now
	item.UpdatedAt = now
	a.channels[channelID] = item
	return nil
}

// restoreWebsiteChannel 恢复内存网站渠道。
func (a *memoryApplication) restoreWebsiteChannel(_ context.Context, principal *servermodels.Principal, channelID string) (*servermodels.Channel, error) {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt == nil {
		return nil, channelaction.ErrNotFound
	}
	item.DeletedAt = nil
	item.UpdatedAt = time.Now()
	a.channels[channelID] = item
	return &item, nil
}

// TestInstallationAndAuthenticationFlow 验证安装、登录和登出的完整 HTTP 流程。
func TestInstallationAndAuthenticationFlow(t *testing.T) {
	application := newMemoryApplication()
	server := httptest.NewServer(newTestService(application))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/inbox", nil), http.StatusConflict, "INSTALLATION_REQUIRED")

	installResponse := doJSON(t, client, http.MethodPost, server.URL+"/install", map[string]string{
		"organizationName": "鹿行测试公司",
		"displayName":      "所有者",
		"email":            "owner@example.com",
		"password":         "password123",
	})
	if installResponse.StatusCode != http.StatusCreated {
		t.Fatalf("install status = %d, want %d", installResponse.StatusCode, http.StatusCreated)
	}
	installResponse.Body.Close()

	inboxResponse := doJSON(t, client, http.MethodGet, server.URL+"/inbox", nil)
	if inboxResponse.StatusCode != http.StatusOK {
		t.Fatalf("authenticated inbox status = %d, want %d", inboxResponse.StatusCode, http.StatusOK)
	}
	inboxResponse.Body.Close()

	logoutResponse := doJSON(t, client, http.MethodPost, server.URL+"/auth/logout", nil)
	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logoutResponse.StatusCode, http.StatusNoContent)
	}
	logoutResponse.Body.Close()

	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/inbox", nil), http.StatusUnauthorized, "AUTH_REQUIRED")
	assertErrorCode(t, doJSON(t, client, http.MethodPost, server.URL+"/auth/login", map[string]string{
		"email": "owner@example.com", "password": "wrong-password",
	}), http.StatusUnauthorized, "INVALID_CREDENTIALS")

	loginResponse := doJSON(t, client, http.MethodPost, server.URL+"/auth/login", map[string]string{
		"email": "OWNER@example.com", "password": "password123",
	})
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.StatusCode, http.StatusOK)
	}
	loginResponse.Body.Close()
}

// TestWebsiteChannelLifecycle 验证网站渠道创建、修改、回收和恢复流程。
func TestWebsiteChannelLifecycle(t *testing.T) {
	application := newMemoryApplication()
	server := httptest.NewServer(newTestService(application))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	installResponse := doJSON(t, client, http.MethodPost, server.URL+"/install", map[string]string{
		"organizationName": "鹿行测试公司",
		"displayName":      "所有者",
		"email":            "owner@example.com",
		"password":         "password123",
	})
	installResponse.Body.Close()
	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website/not-a-uuid", nil), http.StatusNotFound, "CHANNEL_NOT_FOUND")

	createResponse := doJSON(t, client, http.MethodPost, server.URL+"/channels/website", map[string]string{
		"name":          "产品官网",
		"description":   "接收官网访客咨询",
		"defaultLocale": "zh-CN",
	})
	if createResponse.StatusCode != http.StatusCreated {
		createResponse.Body.Close()
		t.Fatalf("create status = %d, want %d", createResponse.StatusCode, http.StatusCreated)
	}
	var created servermodels.Channel
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		createResponse.Body.Close()
		t.Fatal(err)
	}
	createResponse.Body.Close()
	if created.Type != channelaction.TypeWebsite || created.CreatedByUserID != "user-1" {
		t.Fatalf("unexpected created channel: %#v", created)
	}

	updateResponse := doJSON(t, client, http.MethodPatch, server.URL+"/channels/website/"+created.ID, map[string]string{
		"name":          "帮助中心",
		"description":   "",
		"defaultLocale": "en-US",
	})
	if updateResponse.StatusCode != http.StatusOK {
		updateResponse.Body.Close()
		t.Fatalf("update status = %d, want %d", updateResponse.StatusCode, http.StatusOK)
	}
	var updated servermodels.Channel
	if err := json.NewDecoder(updateResponse.Body).Decode(&updated); err != nil {
		updateResponse.Body.Close()
		t.Fatal(err)
	}
	updateResponse.Body.Close()
	if updated.Name != "帮助中心" || updated.Description != nil || updated.DefaultLocale != "en-US" {
		t.Fatalf("unexpected updated channel: %#v", updated)
	}

	deleteResponse := doJSON(t, client, http.MethodDelete, server.URL+"/channels/website/"+created.ID, nil)
	if deleteResponse.StatusCode != http.StatusNoContent {
		deleteResponse.Body.Close()
		t.Fatalf("delete status = %d, want %d", deleteResponse.StatusCode, http.StatusNoContent)
	}
	deleteResponse.Body.Close()
	assertChannelListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website", nil), 0)
	assertChannelListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website/trash", nil), 1)
	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website/"+created.ID, nil), http.StatusNotFound, "CHANNEL_NOT_FOUND")

	restoreResponse := doJSON(t, client, http.MethodPost, server.URL+"/channels/website/"+created.ID+"/restore", nil)
	if restoreResponse.StatusCode != http.StatusOK {
		restoreResponse.Body.Close()
		t.Fatalf("restore status = %d, want %d", restoreResponse.StatusCode, http.StatusOK)
	}
	restoreResponse.Body.Close()
	assertChannelListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website", nil), 1)
	assertChannelListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website/trash", nil), 0)
}

// TestErrorResponseUsesRequestedLanguage 验证 API 错误响应使用请求语言。
func TestErrorResponseUsesRequestedLanguage(t *testing.T) {
	server := httptest.NewServer(newTestService(newMemoryApplication()))
	defer server.Close()

	tests := []struct {
		language string
		message  string
	}{
		{language: "zh-CN", message: "Cervi 尚未完成初始化。"},
		{language: "en-US", message: "Cervi has not been initialized."},
	}

	for _, test := range tests {
		request, err := http.NewRequest(http.MethodGet, server.URL+"/inbox", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Accept-Language", test.language)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var payload errorBody
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			response.Body.Close()
			t.Fatal(err)
		}
		response.Body.Close()
		if payload.Error.Message != test.message {
			t.Fatalf("message = %q, want %q", payload.Error.Message, test.message)
		}
		if response.Header.Get("Content-Language") != test.language {
			t.Fatalf("Content-Language = %q, want %q", response.Header.Get("Content-Language"), test.language)
		}
	}
}

// doJSON 发送测试 JSON 请求并返回响应。
func doJSON(t *testing.T, client *http.Client, method, endpoint string, body any) *http.Response {
	t.Helper()
	var buffer bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buffer).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, endpoint, &buffer)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

// assertErrorCode 校验 API 错误的状态码和业务码。
func assertErrorCode(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	defer response.Body.Close()
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

// assertChannelListSize 校验网站渠道列表长度。
func assertChannelListSize(t *testing.T, response *http.Response, size int) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var payload struct {
		Channels []servermodels.Channel `json:"channels"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Channels) != size {
		t.Fatalf("channel count = %d, want %d", len(payload.Channels), size)
	}
}

// TestSessionCookieUsesSecureFlagBehindHTTPSProxy 验证 HTTPS 代理下的安全 Cookie 标记。
func TestSessionCookieUsesSecureFlagBehindHTTPSProxy(t *testing.T) {
	context, _ := ginTestContext(t)
	request := httptest.NewRequest(http.MethodPost, "/install", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	context.Request = request

	setSessionCookie(context, "token", time.Now().Add(time.Hour))
	response := context.Writer.Header().Values("Set-Cookie")
	if len(response) != 1 || !containsCookieAttribute(response[0], "Secure") || !containsCookieAttribute(response[0], "HttpOnly") {
		t.Fatalf("Set-Cookie = %v, want Secure and HttpOnly", response)
	}
}

// TestLogoutPreservesCookieWhenSessionDeletionFails 验证会话删除失败时保留登录 Cookie。
func TestLogoutPreservesCookieWhenSessionDeletionFails(t *testing.T) {
	application := newMemoryApplication()
	server := httptest.NewServer(newTestService(application))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	installResponse := doJSON(t, client, http.MethodPost, server.URL+"/install", map[string]string{
		"organizationName": "鹿行测试公司",
		"displayName":      "所有者",
		"email":            "owner@example.com",
		"password":         "password123",
	})
	installResponse.Body.Close()

	application.deleteErr = errors.New("database unavailable")
	assertErrorCode(t, doJSON(t, client, http.MethodPost, server.URL+"/auth/logout", nil), http.StatusInternalServerError, "LOGOUT_FAILED")

	inboxResponse := doJSON(t, client, http.MethodGet, server.URL+"/inbox", nil)
	defer inboxResponse.Body.Close()
	if inboxResponse.StatusCode != http.StatusOK {
		t.Fatalf("inbox status after failed logout = %d, want %d", inboxResponse.StatusCode, http.StatusOK)
	}
}

// ginTestContext 创建带响应记录器的 Gin 测试上下文。
func ginTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	return context, recorder
}

// containsCookieAttribute 判断 Cookie 是否包含指定属性。
func containsCookieAttribute(value, attribute string) bool {
	return strings.Contains(value, "; "+attribute)
}
