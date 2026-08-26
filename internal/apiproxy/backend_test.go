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
	_, err = backend.LoadInbox(context.Background(), appservice.RequestMeta{Locale: "zh-CN"})
	var apiError *appservice.Error
	if !errors.As(err, &apiError) || apiError.State != appservice.SessionStateConnect {
		t.Fatalf("error = %#v, want connect session", err)
	}
}

// TestAbsoluteContentURL 验证原生端文件地址指向已连接的企业服务器。
func TestAbsoluteContentURL(t *testing.T) {
	backend, err := newTestBackend(&memoryStore{serverURL: "https://cervi.example.com/company"})
	if err != nil {
		t.Fatal(err)
	}
	if got := backend.absoluteContentURL("/files/file-1/content"); got != "https://cervi.example.com/company/files/file-1/content" {
		t.Fatalf("content URL = %q", got)
	}
	if got := backend.absoluteContentURL("https://storage.example.com/object"); got != "https://storage.example.com/object" {
		t.Fatalf("absolute content URL = %q", got)
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
	_, err = backend.LoadInbox(ctx, appservice.RequestMeta{})
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
			if request.Header.Get("Authorization") != "" {
				http.Error(writer, "installation status must not use login state", http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, map[string]any{"installed": true, "organizationName": "鹿行"})
		case "/api/inbox":
			if request.Header.Get("Authorization") == "Bearer test-token" {
				writeTestJSON(writer, http.StatusOK, map[string]any{
					"organization":  map[string]string{"id": "organization-1", "name": "鹿行"},
					"user":          map[string]string{"id": "user-1", "organizationId": "organization-1", "email": "admin@example.com"},
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
			if input.AvatarFileID != "file-1" {
				http.Error(writer, "unexpected avatar file", http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, map[string]string{
				"id": "user-1", "organizationId": "organization-1", "displayName": input.DisplayName, "email": input.Email,
				"avatarUrl": "/files/file-1/content",
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
			if !input.MessageNotificationsEnabled {
				http.Error(writer, "unexpected notification preference", http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, map[string]any{
				"id": "user-1", "organizationId": "organization-1", "locale": string(input.Locale), "timeZone": input.TimeZone,
				"messageNotificationsEnabled": input.MessageNotificationsEnabled,
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
		case "/api/agents/agent-1/work-status":
			if request.Method != http.MethodPut || request.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(writer, "unexpected agent work status request", http.StatusBadRequest)
				return
			}
			var input appservice.AgentWorkStatusInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, appservice.Agent{ID: "agent-1", WorkStatus: input.WorkStatus, Teams: []appservice.TeamSummary{}})
		case "/api/agents/agent-1/execution":
			if request.Method != http.MethodPut || request.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(writer, "unexpected agent execution request", http.StatusBadRequest)
				return
			}
			var input appservice.AgentExecutionInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, appservice.Agent{
				ID: "agent-1", Teams: []appservice.TeamSummary{},
				Execution: appservice.AgentExecution{RevisionID: "revision-1", Mode: input.Mode, Managed: &appservice.AgentManagedExecution{
					ProviderID: input.Managed.ProviderID, ProviderName: "企业模型",
					ModelIdentifier: input.Managed.ModelIdentifier, ModelName: "对话模型",
					SystemInstruction: input.Managed.SystemInstruction,
				}},
			})
		case "/api/teams/team-1/members":
			if request.Method != http.MethodGet || request.URL.Query().Get("workStatus") != string(appservice.WorkStatusOffDuty) {
				http.Error(writer, "unexpected team member query", http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, appservice.TeamMemberList{Members: []appservice.TeamMember{}, Page: appservice.PageInfo{Number: 1, Size: 50}})
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
		case "/api/settings/organization":
			if request.Method != http.MethodPut || request.Header.Get("Authorization") != "Bearer test-token" {
				writeTestJSON(writer, http.StatusUnauthorized, map[string]any{"error": map[string]string{
					"code": "AUTH_REQUIRED", "message": "Authentication required.",
				}})
				return
			}
			var input appservice.OrganizationInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, appservice.Organization{ID: "organization-1", Name: input.Name})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer remote.Close()

	store := &memoryStore{}
	backend, err := newTestBackend(store)
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
	if err := backend.ConnectServer(context.Background(), meta, remote.URL); err != nil {
		t.Fatal(err)
	}
	if store.serverURL != remote.URL {
		t.Fatalf("server URL = %q, want %q", store.serverURL, remote.URL)
	}
	if err := backend.ConnectServer(context.Background(), meta, remote.URL); err != nil {
		t.Fatal(err)
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
	if err := backend.ConnectServer(context.Background(), meta, remote.URL); err != nil || !store.credentialSet {
		t.Fatalf("same server credential found = %v, error = %v", store.credentialSet, err)
	}
	backend, err = newTestBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.LoadInbox(context.Background(), meta); err != nil {
		t.Fatal(err)
	}
	user, err := backend.UpdateProfile(context.Background(), meta, appservice.ProfileInput{
		DisplayName:  "林晓",
		Email:        "lin@example.com",
		AvatarFileID: "file-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.DisplayName != "林晓" || user.Email != "lin@example.com" || user.AvatarURL != remote.URL+"/files/file-1/content" {
		t.Fatalf("updated user = %#v", user)
	}
	if err := backend.ChangePassword(context.Background(), meta, appservice.ChangePasswordInput{
		CurrentPassword: "password123",
		NewPassword:     "new-password123",
	}); err != nil {
		t.Fatal(err)
	}
	preferences, err := backend.UpdateUserPreferences(context.Background(), meta, appservice.UserPreferencesInput{
		Locale: appservice.LocaleEnglishUnitedStates, TimeZone: "America/New_York", MessageNotificationsEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preferences.Locale != appservice.LocaleEnglishUnitedStates || preferences.TimeZone != "America/New_York" || !preferences.MessageNotificationsEnabled {
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
	agent, err := backend.UpdateAgentWorkStatus(context.Background(), meta, "agent-1", appservice.AgentWorkStatusInput{WorkStatus: appservice.WorkStatusAway})
	if err != nil || agent.WorkStatus != appservice.WorkStatusAway {
		t.Fatalf("updated agent work status = %#v, error = %v", agent, err)
	}
	agent, err = backend.UpdateAgentExecution(context.Background(), meta, "agent-1", appservice.AgentExecutionInput{
		Mode: appservice.AgentExecutionModeManaged,
		Managed: &appservice.AgentManagedExecutionInput{
			ProviderID: "provider-1", ModelIdentifier: "chat-model", SystemInstruction: "回答产品问题。",
		},
	})
	if err != nil || agent.Execution.Managed == nil || agent.Execution.Managed.ModelIdentifier != "chat-model" {
		t.Fatalf("updated agent execution = %#v, error = %v", agent.Execution, err)
	}
	offDuty := appservice.WorkStatusOffDuty
	teamMembers, err := backend.ListTeamMembers(context.Background(), meta, "team-1", appservice.TeamMemberListInput{WorkStatus: &offDuty, Page: 1, PageSize: 50})
	if err != nil || teamMembers.Page.Number != 1 || teamMembers.Page.Size != 50 {
		t.Fatalf("team members = %#v, error = %v", teamMembers, err)
	}
	organization, err := backend.UpdateOrganization(context.Background(), meta, appservice.OrganizationInput{Name: "鹿行协作"})
	if err != nil || organization.Name != "鹿行协作" {
		t.Fatalf("updated organization = %#v, error = %v", organization, err)
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
	_, err = backend.LoadInbox(context.Background(), appservice.RequestMeta{Locale: "zh-CN"})
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
