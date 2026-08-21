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
	lastMeta         appservice.RequestMeta
	lastOrganization appservice.OrganizationInput
	lastUserList     appservice.UserListInput
	lastCreateUser   appservice.CreateUserInput
	lastUpdateUser   appservice.UpdateDirectoryUserInput
	lastTeamList     appservice.TeamListInput
	lastTeam         appservice.TeamInput
	lastIdentityType appservice.MemberIdentityType
	lastIdentityID   string
	lastProfile      appservice.ProfileInput
	lastFileUpload   appservice.FileUploadInput
	lastFileID       string
	lastPassword     appservice.ChangePasswordInput
	lastPreferences  appservice.UserPreferencesInput
	lastWorkStatus   appservice.UserWorkStatusInput
}

func (b *testBackend) InstallationStatus(context.Context, appservice.RequestMeta) (appservice.InstallationStatus, error) {
	return appservice.InstallationStatus{Installed: true, OrganizationName: "鹿行"}, nil
}

func (b *testBackend) Login(_ context.Context, meta appservice.RequestMeta, input appservice.LoginInput) (appservice.Auth, error) {
	b.lastMeta = meta
	if input.Email != "admin@example.com" || input.Password != "password123" {
		return appservice.Auth{}, &appservice.Error{Kind: appservice.ErrorKindInvalid, Message: "账号或密码错误。"}
	}
	return appservice.Auth{
		Identity:  testIdentity(),
		Token:     "test-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (b *testBackend) LoadIdentity(_ context.Context, meta appservice.RequestMeta) (appservice.Identity, error) {
	b.lastMeta = meta
	if meta.Token != "test-token" {
		return appservice.Identity{}, &appservice.Error{State: appservice.SessionStateLogin, Message: "请先登录。"}
	}
	return testIdentity(), nil
}

// UpdateProfile 记录个人资料输入并返回更新后的用户。
func (b *testBackend) UpdateProfile(_ context.Context, meta appservice.RequestMeta, input appservice.ProfileInput) (appservice.User, error) {
	b.lastMeta = meta
	b.lastProfile = input
	identity := testIdentity()
	identity.User.DisplayName = input.DisplayName
	identity.User.Email = input.Email
	return identity.User, nil
}

// CreateFileUpload 记录文件上传输入并返回上传请求。
func (b *testBackend) CreateFileUpload(_ context.Context, meta appservice.RequestMeta, input appservice.FileUploadInput) (appservice.FileUpload, error) {
	b.lastMeta = meta
	b.lastFileUpload = input
	return appservice.FileUpload{File: appservice.File{ID: "file-1"}, Request: appservice.FileUploadRequest{Method: http.MethodPut, URL: "/files/file-1/content"}}, nil
}

// CompleteFileUpload 记录完成上传的文件编号。
func (b *testBackend) CompleteFileUpload(_ context.Context, meta appservice.RequestMeta, fileID string) (appservice.File, error) {
	b.lastMeta = meta
	b.lastFileID = fileID
	return appservice.File{ID: fileID}, nil
}

// ChangePassword 记录修改密码输入。
func (b *testBackend) ChangePassword(_ context.Context, meta appservice.RequestMeta, input appservice.ChangePasswordInput) error {
	b.lastMeta = meta
	b.lastPassword = input
	return nil
}

// UpdateUserPreferences 记录语言和时区输入并返回更新后的用户。
func (b *testBackend) UpdateUserPreferences(_ context.Context, meta appservice.RequestMeta, input appservice.UserPreferencesInput) (appservice.User, error) {
	b.lastMeta = meta
	b.lastPreferences = input
	identity := testIdentity()
	identity.User.Locale = input.Locale
	identity.User.TimeZone = input.TimeZone
	return identity.User, nil
}

// UpdateUserWorkStatus 记录工作状态输入并返回更新后的用户。
func (b *testBackend) UpdateUserWorkStatus(_ context.Context, meta appservice.RequestMeta, input appservice.UserWorkStatusInput) (appservice.User, error) {
	b.lastMeta = meta
	b.lastWorkStatus = input
	identity := testIdentity()
	identity.User.WorkStatus = input.WorkStatus
	return identity.User, nil
}

func (b *testBackend) ListUsers(_ context.Context, meta appservice.RequestMeta, input appservice.UserListInput) (appservice.UserList, error) {
	b.lastMeta = meta
	b.lastUserList = input
	return appservice.UserList{Users: []appservice.DirectoryUser{}, Page: appservice.PageInfo{Number: input.Page, Size: input.PageSize}}, nil
}

func (b *testBackend) CreateUser(_ context.Context, meta appservice.RequestMeta, input appservice.CreateUserInput) (appservice.DirectoryUser, error) {
	b.lastMeta = meta
	b.lastCreateUser = input
	return appservice.DirectoryUser{ID: "user-2", DisplayName: input.DisplayName, Email: input.Email, Role: appservice.RoleSummary{ID: input.RoleID}, Status: appservice.UserStatusActive, Teams: []appservice.TeamSummary{}}, nil
}

func (b *testBackend) UpdateUser(_ context.Context, meta appservice.RequestMeta, userID string, input appservice.UpdateDirectoryUserInput) (appservice.DirectoryUser, error) {
	b.lastMeta = meta
	b.lastUpdateUser = input
	return appservice.DirectoryUser{ID: userID, DisplayName: input.DisplayName, Email: input.Email, Role: appservice.RoleSummary{ID: input.RoleID}, Status: appservice.UserStatusActive, Teams: []appservice.TeamSummary{}}, nil
}

func (b *testBackend) ListTeams(_ context.Context, meta appservice.RequestMeta, input appservice.TeamListInput) (appservice.TeamList, error) {
	b.lastMeta = meta
	b.lastTeamList = input
	return appservice.TeamList{Teams: []appservice.Team{}, Page: appservice.PageInfo{Number: input.Page, Size: input.PageSize}}, nil
}

func (b *testBackend) CreateTeam(_ context.Context, meta appservice.RequestMeta, input appservice.TeamInput) (appservice.Team, error) {
	b.lastMeta = meta
	b.lastTeam = input
	return appservice.Team{ID: "team-1", Name: input.Name, Description: input.Description}, nil
}

// RemoveTeamMember 记录测试中的团队身份移出参数。
func (b *testBackend) RemoveTeamMember(_ context.Context, meta appservice.RequestMeta, _ string, identityType appservice.MemberIdentityType, identityID string) error {
	b.lastMeta = meta
	b.lastIdentityType = identityType
	b.lastIdentityID = identityID
	return nil
}

// UpdateOrganization 保存测试企业名称。
func (b *testBackend) UpdateOrganization(_ context.Context, meta appservice.RequestMeta, input appservice.OrganizationInput) (appservice.Organization, error) {
	b.lastMeta = meta
	b.lastOrganization = input
	return appservice.Organization{ID: testIdentity().Organization.ID, Name: input.Name}, nil
}

// TestAuthenticationUsesBearerToken 验证登录返回令牌且后续请求读取 Bearer Token。
func TestAuthenticationUsesBearerToken(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	loginResponse := doJSON(t, http.MethodPost, server.URL+"/auth/login", map[string]string{
		"email": "admin@example.com", "password": "password123",
	}, "")
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.StatusCode, http.StatusOK)
	}
	var auth appservice.Auth
	if err := json.NewDecoder(loginResponse.Body).Decode(&auth); err != nil {
		t.Fatal(err)
	}
	if auth.Token != "test-token" {
		t.Fatalf("token = %q, want test-token", auth.Token)
	}

	unauthorized := doJSON(t, http.MethodGet, server.URL+"/auth/identity", nil, "")
	assertError(t, unauthorized, http.StatusUnauthorized, "", appservice.SessionStateLogin)

	authorized := doJSON(t, http.MethodGet, server.URL+"/auth/identity", nil, auth.Token)
	defer authorized.Body.Close()
	if authorized.StatusCode != http.StatusOK {
		t.Fatalf("identity status = %d, want %d", authorized.StatusCode, http.StatusOK)
	}
	if backend.lastMeta.Token != auth.Token {
		t.Fatalf("backend token = %q, want %q", backend.lastMeta.Token, auth.Token)
	}
}

// TestInstallationStatusReturnsOrganizationName 验证未登录可读取公开企业名称。
func TestInstallationStatusReturnsOrganizationName(t *testing.T) {
	server := httptest.NewServer(NewService(appservice.New(&testBackend{})))
	defer server.Close()

	response := doJSON(t, http.MethodGet, server.URL+"/installation/status", nil, "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var status appservice.InstallationStatus
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.OrganizationName != "鹿行" {
		t.Fatalf("status = %#v", status)
	}
}

// TestListQueryIsConvertedToTypedInput 验证 HTTP 查询参数转换为类型化服务输入。
func TestListQueryIsConvertedToTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodGet, server.URL+"/users?query=lin&status=active&roleId=0198ddee-c056-7bc5-a1d9-586f878ee966&teamId=0198ddee-c056-7bc5-a1d9-586f878ee965&page=2&pageSize=25", nil, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if backend.lastUserList.Query != "lin" || backend.lastUserList.Status == nil || *backend.lastUserList.Status != "active" || backend.lastUserList.RoleID == "" || backend.lastUserList.TeamID == "" || backend.lastUserList.Page != 2 || backend.lastUserList.PageSize != 25 {
		t.Fatalf("typed input = %#v", backend.lastUserList)
	}
}

// TestMemberAndTeamMutationsUseTypedContracts 验证成员和团队写入使用类型化契约。
func TestMemberAndTeamMutationsUseTypedContracts(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	memberResponse := doJSON(t, http.MethodPost, server.URL+"/users", appservice.CreateUserInput{
		DisplayName: "林晓", Email: "lin@example.com", Password: "password123", RoleID: "0198ddee-c056-7bc5-a1d9-586f878ee966", TeamIDs: []string{"team-1"},
	}, "test-token")
	defer memberResponse.Body.Close()
	if memberResponse.StatusCode != http.StatusCreated || backend.lastCreateUser.DisplayName != "林晓" || len(backend.lastCreateUser.TeamIDs) != 1 {
		t.Fatalf("status = %d, input = %#v", memberResponse.StatusCode, backend.lastCreateUser)
	}

	teamResponse := doJSON(t, http.MethodPost, server.URL+"/teams", appservice.TeamInput{Name: "客户成功", Description: "服务客户"}, "test-token")
	defer teamResponse.Body.Close()
	if teamResponse.StatusCode != http.StatusCreated || backend.lastTeam.Name != "客户成功" {
		t.Fatalf("status = %d, input = %#v", teamResponse.StatusCode, backend.lastTeam)
	}

	listResponse := doJSON(t, http.MethodGet, server.URL+"/teams?query=客户&page=2&pageSize=20", nil, "test-token")
	defer listResponse.Body.Close()
	if listResponse.StatusCode != http.StatusOK || backend.lastTeamList.Query != "客户" || backend.lastTeamList.Page != 2 || backend.lastTeamList.PageSize != 20 {
		t.Fatalf("status = %d, input = %#v", listResponse.StatusCode, backend.lastTeamList)
	}

	removeResponse := doJSON(t, http.MethodDelete, server.URL+"/teams/team-1/members/agent/agent-2", nil, "test-token")
	defer removeResponse.Body.Close()
	if removeResponse.StatusCode != http.StatusNoContent || backend.lastIdentityType != appservice.MemberIdentityTypeAgent || backend.lastIdentityID != "agent-2" {
		t.Fatalf("status = %d, identity type = %q, identity id = %q", removeResponse.StatusCode, backend.lastIdentityType, backend.lastIdentityID)
	}
}

// TestUpdateProfileUsesTypedInput 验证个人资料请求转换为类型化服务输入。
func TestUpdateProfileUsesTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodPatch, server.URL+"/profile", appservice.ProfileInput{
		DisplayName:  "林晓",
		Email:        "lin@example.com",
		AvatarFileID: "file-1",
	}, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if backend.lastMeta.Token != "test-token" || backend.lastProfile.DisplayName != "林晓" || backend.lastProfile.Email != "lin@example.com" || backend.lastProfile.AvatarFileID != "file-1" {
		t.Fatalf("profile input = %#v, meta = %#v", backend.lastProfile, backend.lastMeta)
	}
}

// TestFileUploadRoutesUseTypedInput 验证文件上传接口使用统一契约。
func TestFileUploadRoutesUseTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	created := doJSON(t, http.MethodPost, server.URL+"/files/uploads", appservice.FileUploadInput{
		Purpose: appservice.FilePurposeUserAvatar, FileName: "avatar.png", ContentType: "image/png", ByteSize: 1024,
	}, "test-token")
	created.Body.Close()
	if created.StatusCode != http.StatusCreated || backend.lastFileUpload.FileName != "avatar.png" {
		t.Fatalf("create status = %d, input = %#v", created.StatusCode, backend.lastFileUpload)
	}

	completed := doJSON(t, http.MethodPost, server.URL+"/files/file-1/complete", nil, "test-token")
	completed.Body.Close()
	if completed.StatusCode != http.StatusOK || backend.lastFileID != "file-1" {
		t.Fatalf("complete status = %d, file ID = %q", completed.StatusCode, backend.lastFileID)
	}
}

// TestChangePasswordUsesTypedInput 验证修改密码请求转换为类型化服务输入。
func TestChangePasswordUsesTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodPatch, server.URL+"/password", appservice.ChangePasswordInput{
		CurrentPassword: "password123",
		NewPassword:     "new-password123",
	}, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if backend.lastMeta.Token != "test-token" || backend.lastPassword.CurrentPassword != "password123" || backend.lastPassword.NewPassword != "new-password123" {
		t.Fatalf("password input = %#v, meta = %#v", backend.lastPassword, backend.lastMeta)
	}
}

// TestUpdateUserPreferencesUsesTypedInput 验证语言和时区请求转换为类型化服务输入。
func TestUpdateUserPreferencesUsesTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodPatch, server.URL+"/preferences", appservice.UserPreferencesInput{
		Locale:   appservice.LocaleEnglishUnitedStates,
		TimeZone: "America/New_York",
	}, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if backend.lastMeta.Token != "test-token" || backend.lastPreferences.Locale != appservice.LocaleEnglishUnitedStates || backend.lastPreferences.TimeZone != "America/New_York" {
		t.Fatalf("preferences input = %#v, meta = %#v", backend.lastPreferences, backend.lastMeta)
	}
}

// TestUpdateUserWorkStatusUsesTypedInput 验证工作状态请求转换为类型化服务输入。
func TestUpdateUserWorkStatusUsesTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodPatch, server.URL+"/work-status", appservice.UserWorkStatusInput{
		WorkStatus: appservice.WorkStatusAway,
	}, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if backend.lastMeta.Token != "test-token" || backend.lastWorkStatus.WorkStatus != appservice.WorkStatusAway {
		t.Fatalf("work status input = %#v, meta = %#v", backend.lastWorkStatus, backend.lastMeta)
	}
}

// TestOrganizationSettingsUseTypedContract 验证企业信息接口保存类型化契约。
func TestOrganizationSettingsUseTypedContract(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	updateResponse := doJSON(t, http.MethodPut, server.URL+"/settings/organization", appservice.OrganizationInput{Name: "鹿行协作"}, "test-token")
	defer updateResponse.Body.Close()
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want %d", updateResponse.StatusCode, http.StatusOK)
	}
	var organization appservice.Organization
	if err := json.NewDecoder(updateResponse.Body).Decode(&organization); err != nil {
		t.Fatal(err)
	}
	if organization.Name != "鹿行协作" || backend.lastMeta.Token != "test-token" || backend.lastOrganization.Name != "鹿行协作" {
		t.Fatalf("organization = %#v, meta = %#v", organization, backend.lastMeta)
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
	assertError(t, response, http.StatusBadRequest, appservice.ErrorKindInvalid, "")
	if response.Header.Get("Content-Language") != "zh-CN" {
		t.Fatalf("language = %q, want zh-CN", response.Header.Get("Content-Language"))
	}
}

// TestBearerTokenParsing 验证 Bearer Token 请求头解析规则。
func TestBearerTokenParsing(t *testing.T) {
	if token := bearerToken("Bearer test-token"); token != "test-token" {
		t.Fatalf("token = %q, want test-token", token)
	}
	if token := bearerToken("Basic test-token"); token != "" {
		t.Fatalf("basic token = %q, want empty", token)
	}
}

func testIdentity() appservice.Identity {
	return appservice.Identity{
		Organization: appservice.Organization{ID: "organization-1", Name: "鹿行"},
		User:         appservice.User{ID: "user-1", OrganizationID: "organization-1", Email: "admin@example.com", DisplayName: "管理员", RoleID: "role-1", Status: "active", Locale: "zh-CN", TimeZone: "Asia/Shanghai", WorkStatus: appservice.WorkStatusWorking},
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

func assertError(t *testing.T, response *http.Response, status int, kind appservice.ErrorKind, state appservice.SessionState) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != status {
		t.Fatalf("status = %d, want %d", response.StatusCode, status)
	}
	var payload errorBody
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Kind != kind || payload.Error.State != state {
		t.Fatalf("error = %+v, want kind %q state %q", payload.Error, kind, state)
	}
}
