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
	lastMeta             appservice.RequestMeta
	lastOrganization     appservice.OrganizationInput
	lastUserList         appservice.UserListInput
	lastCreateUser       appservice.CreateUserInput
	lastUpdateUser       appservice.UpdateUserInput
	lastRoleChanges      appservice.UserRoleChangesInput
	lastTeamList         appservice.TeamListInput
	lastTeamMemberList   appservice.TeamMemberListInput
	lastTeam             appservice.TeamInput
	lastTeamMembers      appservice.TeamMemberInput
	lastAgentID          string
	lastAgentExecution   appservice.AgentExecutionInput
	lastAgentWorkStatus  appservice.AgentWorkStatusInput
	lastProfile          appservice.ProfileInput
	lastFileUpload       appservice.FileUploadInput
	lastFileID           string
	lastPassword         appservice.ChangePasswordInput
	lastPreferences      appservice.UserPreferencesInput
	lastWorkStatus       appservice.UserWorkStatusInput
	lastConversationID   string
	lastConversationList appservice.ConversationMessageListInput
	lastCustomerMessage  appservice.CustomerTextMessageInput
	lastDocumentBaseID   string
	lastDocumentQuery    appservice.KnowledgeDocumentListInput
	lastAIConnection     appservice.AIProviderConnectionInput
	lastBusinessSystemID string
	lastBusinessSystem   appservice.BusinessSystemInput
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

// ListConversationMessages 记录成员消息查询输入。
func (b *testBackend) ListConversationMessages(_ context.Context, meta appservice.RequestMeta, conversationID string, input appservice.ConversationMessageListInput) (appservice.ConversationMessageList, error) {
	b.lastMeta = meta
	b.lastConversationID = conversationID
	b.lastConversationList = input
	return appservice.ConversationMessageList{Messages: []appservice.ConversationMessage{}}, nil
}

// SendCustomerTextMessage 记录成员客户消息输入。
func (b *testBackend) SendCustomerTextMessage(_ context.Context, meta appservice.RequestMeta, conversationID string, input appservice.CustomerTextMessageInput) (appservice.ConversationMessage, error) {
	b.lastMeta = meta
	b.lastConversationID = conversationID
	b.lastCustomerMessage = input
	return appservice.ConversationMessage{ID: input.ClientMessageID, Type: appservice.MessageTypeText, Body: input.Body}, nil
}

// ListKnowledgeDocuments 记录知识文档查询输入。
func (b *testBackend) ListKnowledgeDocuments(_ context.Context, meta appservice.RequestMeta, knowledgeBaseID string, input appservice.KnowledgeDocumentListInput) (appservice.KnowledgeDocumentList, error) {
	b.lastMeta = meta
	b.lastDocumentBaseID = knowledgeBaseID
	b.lastDocumentQuery = input
	return appservice.KnowledgeDocumentList{Documents: []appservice.KnowledgeDocumentSummary{}, Page: appservice.PageInfo{Number: input.Page, Size: input.PageSize}}, nil
}

// UpdateProfile 记录个人资料输入并返回更新后的用户。
func (b *testBackend) UpdateProfile(_ context.Context, meta appservice.RequestMeta, input appservice.ProfileInput) (appservice.CurrentUser, error) {
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

// UpdateUserPreferences 记录用户偏好输入并返回更新后的用户。
func (b *testBackend) UpdateUserPreferences(_ context.Context, meta appservice.RequestMeta, input appservice.UserPreferencesInput) (appservice.CurrentUser, error) {
	b.lastMeta = meta
	b.lastPreferences = input
	identity := testIdentity()
	identity.User.Locale = input.Locale
	identity.User.TimeZone = input.TimeZone
	identity.User.MessageNotificationsEnabled = input.MessageNotificationsEnabled
	identity.User.WorkspaceTabsEnabled = input.WorkspaceTabsEnabled
	return identity.User, nil
}

// UpdateUserWorkStatus 记录工作状态输入并返回更新后的用户。
func (b *testBackend) UpdateUserWorkStatus(_ context.Context, meta appservice.RequestMeta, input appservice.UserWorkStatusInput) (appservice.CurrentUser, error) {
	b.lastMeta = meta
	b.lastWorkStatus = input
	identity := testIdentity()
	identity.User.WorkStatus = input.WorkStatus
	return identity.User, nil
}

func (b *testBackend) ListUsers(_ context.Context, meta appservice.RequestMeta, input appservice.UserListInput) (appservice.UserList, error) {
	b.lastMeta = meta
	b.lastUserList = input
	return appservice.UserList{Users: []appservice.User{}, Page: appservice.PageInfo{Number: input.Page, Size: input.PageSize}}, nil
}

func (b *testBackend) CreateUser(_ context.Context, meta appservice.RequestMeta, input appservice.CreateUserInput) (appservice.User, error) {
	b.lastMeta = meta
	b.lastCreateUser = input
	return appservice.User{ID: "user-2", DisplayName: input.DisplayName, Email: input.Email, Role: appservice.RoleSummary{ID: input.RoleID}, Status: appservice.UserStatusActive, Teams: []appservice.TeamSummary{}}, nil
}

func (b *testBackend) UpdateUser(_ context.Context, meta appservice.RequestMeta, userID string, input appservice.UpdateUserInput) (appservice.User, error) {
	b.lastMeta = meta
	b.lastUpdateUser = input
	return appservice.User{ID: userID, DisplayName: input.DisplayName, Email: input.Email, Role: appservice.RoleSummary{ID: input.RoleID}, Status: appservice.UserStatusActive, Teams: []appservice.TeamSummary{}}, nil
}

// UpdateUserRoles 记录批量角色调整输入。
func (b *testBackend) UpdateUserRoles(_ context.Context, meta appservice.RequestMeta, input appservice.UserRoleChangesInput) error {
	b.lastMeta = meta
	b.lastRoleChanges = input
	return nil
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

// ListTeamMembers 记录团队成员查询条件。
func (b *testBackend) ListTeamMembers(_ context.Context, meta appservice.RequestMeta, _ string, input appservice.TeamMemberListInput) (appservice.TeamMemberList, error) {
	b.lastMeta = meta
	b.lastTeamMemberList = input
	return appservice.TeamMemberList{Members: []appservice.TeamMember{}, Page: appservice.PageInfo{Number: input.Page, Size: input.PageSize}}, nil
}

// UpdateAgentWorkStatus 记录 AI 员工工作状态输入。
func (b *testBackend) UpdateAgentWorkStatus(_ context.Context, meta appservice.RequestMeta, agentID string, input appservice.AgentWorkStatusInput) (appservice.Agent, error) {
	b.lastMeta = meta
	b.lastAgentID = agentID
	b.lastAgentWorkStatus = input
	return appservice.Agent{ID: agentID, WorkStatus: input.WorkStatus, Teams: []appservice.TeamSummary{}}, nil
}

// UpdateAgentExecution 记录 AI 员工执行配置输入。
func (b *testBackend) UpdateAgentExecution(_ context.Context, meta appservice.RequestMeta, agentID string, input appservice.AgentExecutionInput) (appservice.Agent, error) {
	b.lastMeta = meta
	b.lastAgentID = agentID
	b.lastAgentExecution = input
	return appservice.Agent{ID: agentID, Execution: appservice.AgentExecution{Mode: input.Mode}, Teams: []appservice.TeamSummary{}}, nil
}

// ListTeamMemberCandidates 返回测试中的空候选列表。
func (b *testBackend) ListTeamMemberCandidates(_ context.Context, _ appservice.RequestMeta, _ string, input appservice.TeamMemberCandidateInput) (appservice.TeamMemberCandidateList, error) {
	return appservice.TeamMemberCandidateList{Members: []appservice.TeamMemberCandidate{}, Page: appservice.PageInfo{Number: input.Page, Size: input.PageSize}}, nil
}

// AddTeamMembers 返回测试中的团队。
func (b *testBackend) AddTeamMembers(_ context.Context, meta appservice.RequestMeta, teamID string, input appservice.TeamMemberInput) (appservice.Team, error) {
	b.lastMeta = meta
	b.lastTeamMembers = input
	return appservice.Team{ID: teamID}, nil
}

// RemoveTeamMembers 返回测试中的团队。
func (b *testBackend) RemoveTeamMembers(_ context.Context, meta appservice.RequestMeta, teamID string, input appservice.TeamMemberInput) (appservice.Team, error) {
	b.lastMeta = meta
	b.lastTeamMembers = input
	return appservice.Team{ID: teamID}, nil
}

// UpdateOrganization 保存测试企业通用设置。
func (b *testBackend) UpdateOrganization(_ context.Context, meta appservice.RequestMeta, input appservice.OrganizationInput) (appservice.Organization, error) {
	b.lastMeta = meta
	b.lastOrganization = input
	return appservice.Organization{
		ID: testIdentity().Organization.ID, Name: input.Name, AllowArbitraryURL: input.AllowArbitraryURL,
	}, nil
}

// TestAIProviderConnection 记录模型服务连接测试输入。
func (b *testBackend) TestAIProviderConnection(_ context.Context, meta appservice.RequestMeta, input appservice.AIProviderConnectionInput) error {
	b.lastMeta = meta
	b.lastAIConnection = input
	return nil
}

// ListBusinessSystems 返回测试业务系统列表。
func (b *testBackend) ListBusinessSystems(_ context.Context, meta appservice.RequestMeta) (appservice.BusinessSystemList, error) {
	b.lastMeta = meta
	return appservice.BusinessSystemList{BusinessSystems: []appservice.BusinessSystem{{
		ID: "business-system-1", Name: "企业 ERP", URL: "https://erp.example.com", Enabled: true,
	}}}, nil
}

// GetBusinessSystem 返回测试业务系统详情。
func (b *testBackend) GetBusinessSystem(_ context.Context, meta appservice.RequestMeta, businessSystemID string) (appservice.BusinessSystem, error) {
	b.lastMeta = meta
	b.lastBusinessSystemID = businessSystemID
	return appservice.BusinessSystem{ID: businessSystemID, Name: "企业 ERP", URL: "https://erp.example.com", Enabled: true}, nil
}

// CreateBusinessSystem 记录业务系统创建输入。
func (b *testBackend) CreateBusinessSystem(_ context.Context, meta appservice.RequestMeta, input appservice.BusinessSystemInput) (appservice.BusinessSystem, error) {
	b.lastMeta = meta
	b.lastBusinessSystem = input
	return appservice.BusinessSystem{ID: "business-system-1", Name: input.Name, URL: input.URL, Enabled: input.Enabled}, nil
}

// UpdateBusinessSystem 记录业务系统修改输入。
func (b *testBackend) UpdateBusinessSystem(_ context.Context, meta appservice.RequestMeta, businessSystemID string, input appservice.BusinessSystemInput) (appservice.BusinessSystem, error) {
	b.lastMeta = meta
	b.lastBusinessSystemID = businessSystemID
	b.lastBusinessSystem = input
	return appservice.BusinessSystem{ID: businessSystemID, Name: input.Name, URL: input.URL, Enabled: input.Enabled}, nil
}

// DeleteBusinessSystem 记录业务系统删除编号。
func (b *testBackend) DeleteBusinessSystem(_ context.Context, meta appservice.RequestMeta, businessSystemID string) error {
	b.lastMeta = meta
	b.lastBusinessSystemID = businessSystemID
	return nil
}

// TestAIProviderConnectionUsesDraftContract 验证模型服务连接测试不依赖已保存记录。
func TestAIProviderConnectionUsesDraftContract(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	input := appservice.AIProviderConnectionInput{
		Brand: appservice.AIProviderBrandOpenAI, APIKey: "test-key", APIURL: "https://api.openai.com/v1",
	}
	response := doJSON(t, http.MethodPost, server.URL+"/integrations/model-services/test", input, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent || backend.lastAIConnection != input {
		t.Fatalf("status = %d, input = %#v", response.StatusCode, backend.lastAIConnection)
	}
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

// TestKnowledgeDocumentQueryUsesTypedContract 验证知识文档搜索和状态筛选转换为类型化输入。
func TestKnowledgeDocumentQueryUsesTypedContract(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	const knowledgeBaseID = "0198ddee-c056-7bc5-a1d9-586f878ee966"
	response := doJSON(t, http.MethodGet, server.URL+"/knowledge-bases/"+knowledgeBaseID+"/documents?keyword=产品&status=ready&page=2&pageSize=10", nil, "test-token")
	defer response.Body.Close()
	input := backend.lastDocumentQuery
	if response.StatusCode != http.StatusOK || backend.lastDocumentBaseID != knowledgeBaseID || input.Keyword != "产品" ||
		input.Status == nil || *input.Status != appservice.KnowledgeDocumentStatusReady || input.Page != 2 || input.PageSize != 10 {
		t.Fatalf("status = %d, knowledge base = %q, input = %#v", response.StatusCode, backend.lastDocumentBaseID, input)
	}
}

// TestConversationMessageQueryUsesTypedCursors 验证成员消息路径和游标转换为类型化输入。
func TestConversationMessageQueryUsesTypedCursors(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	const conversationID = "0198ddee-c056-7bc5-a1d9-586f878ee966"
	const before = "0198ddee-c056-7bc5-a1d9-586f878ee966.1787992200123456789.0198ddf0-a234-7f01-8d99-e3e0af0f5f65"
	response := doJSON(t, http.MethodGet, server.URL+"/conversations/"+conversationID+"/messages?before="+before, nil, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || backend.lastConversationID != conversationID || backend.lastConversationList.Before != before || backend.lastConversationList.After != "" {
		t.Fatalf("status = %d, conversation = %q, input = %#v", response.StatusCode, backend.lastConversationID, backend.lastConversationList)
	}
}

// TestCustomerTextMessageUsesTypedContract 验证成员回复路径和请求体转换为类型化输入。
func TestCustomerTextMessageUsesTypedContract(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	const conversationID = "0198ddee-c056-7bc5-a1d9-586f878ee966"
	input := appservice.CustomerTextMessageInput{
		ClientMessageID: "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
		Body:            "回复客户",
	}
	response := doJSON(t, http.MethodPost, server.URL+"/conversations/"+conversationID+"/messages", input, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || backend.lastConversationID != conversationID || backend.lastCustomerMessage != input {
		t.Fatalf("status = %d, conversation = %q, input = %#v", response.StatusCode, backend.lastConversationID, backend.lastCustomerMessage)
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

	roleResponse := doJSON(t, http.MethodPatch, server.URL+"/users/roles", appservice.UserRoleChangesInput{Changes: []appservice.UserRoleChangeInput{{UserID: "0198ddee-c056-7bc5-a1d9-586f878ee967", RoleID: "0198ddee-c056-7bc5-a1d9-586f878ee966"}}}, "test-token")
	defer roleResponse.Body.Close()
	if roleResponse.StatusCode != http.StatusNoContent || len(backend.lastRoleChanges.Changes) != 1 || backend.lastRoleChanges.Changes[0].UserID == "" {
		t.Fatalf("status = %d, input = %#v", roleResponse.StatusCode, backend.lastRoleChanges)
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

	addResponse := doJSON(t, http.MethodPost, server.URL+"/teams/team-1/members", appservice.TeamMemberInput{Members: []appservice.TeamMemberIdentityInput{{IdentityType: appservice.OrganizationIdentityTypeUser, IdentityID: "0198ddee-c056-7bc5-a1d9-586f878ee967"}}}, "test-token")
	defer addResponse.Body.Close()
	if addResponse.StatusCode != http.StatusOK || len(backend.lastTeamMembers.Members) != 1 || backend.lastTeamMembers.Members[0].IdentityType != appservice.OrganizationIdentityTypeUser {
		t.Fatalf("status = %d, input = %#v", addResponse.StatusCode, backend.lastTeamMembers)
	}

	removeResponse := doJSON(t, http.MethodPost, server.URL+"/teams/team-1/members/remove", appservice.TeamMemberInput{Members: []appservice.TeamMemberIdentityInput{{IdentityType: appservice.OrganizationIdentityTypeAgent, IdentityID: "0198ddee-c056-7bc5-a1d9-586f878ee967"}}}, "test-token")
	defer removeResponse.Body.Close()
	if removeResponse.StatusCode != http.StatusOK || len(backend.lastTeamMembers.Members) != 1 || backend.lastTeamMembers.Members[0].IdentityType != appservice.OrganizationIdentityTypeAgent {
		t.Fatalf("status = %d, input = %#v", removeResponse.StatusCode, backend.lastTeamMembers)
	}
}

// TestTeamMemberWorkStatusQueryUsesTypedContract 验证团队成员列表按工作状态查询。
func TestTeamMemberWorkStatusQueryUsesTypedContract(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	teamResponse := doJSON(t, http.MethodGet, server.URL+"/teams/team-1/members?workStatus=off_duty&page=3&pageSize=10", nil, "test-token")
	defer teamResponse.Body.Close()
	if teamResponse.StatusCode != http.StatusOK || backend.lastTeamMemberList.WorkStatus == nil || *backend.lastTeamMemberList.WorkStatus != appservice.WorkStatusOffDuty || backend.lastTeamMemberList.Page != 3 || backend.lastTeamMemberList.PageSize != 10 {
		t.Fatalf("team status = %d, input = %#v", teamResponse.StatusCode, backend.lastTeamMemberList)
	}
}

// TestUpdateAgentWorkStatusUsesTypedInput 验证 AI 员工工作状态请求转换为类型化服务输入。
func TestUpdateAgentWorkStatusUsesTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodPut, server.URL+"/agents/agent-1/work-status", appservice.AgentWorkStatusInput{WorkStatus: appservice.WorkStatusAway}, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || backend.lastAgentID != "agent-1" || backend.lastAgentWorkStatus.WorkStatus != appservice.WorkStatusAway {
		t.Fatalf("status = %d, agent = %q, input = %#v", response.StatusCode, backend.lastAgentID, backend.lastAgentWorkStatus)
	}
}

// TestUpdateAgentExecutionUsesTypedInput 验证 AI 员工执行配置请求转换为类型化服务输入。
func TestUpdateAgentExecutionUsesTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodPut, server.URL+"/agents/agent-1/execution", appservice.AgentExecutionInput{
		Mode: appservice.AgentExecutionModeManaged,
		Managed: &appservice.AgentManagedExecutionInput{
			ProviderID: "provider-1", ModelIdentifier: "chat-model", SystemInstruction: "回答产品问题。",
		},
	}, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || backend.lastAgentID != "agent-1" || backend.lastAgentExecution.Mode != appservice.AgentExecutionModeManaged || backend.lastAgentExecution.Managed == nil || backend.lastAgentExecution.Managed.ModelIdentifier != "chat-model" {
		t.Fatalf("status = %d, agent = %q, input = %#v", response.StatusCode, backend.lastAgentID, backend.lastAgentExecution)
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

// TestUpdateUserPreferencesUsesTypedInput 验证用户偏好请求转换为类型化服务输入。
func TestUpdateUserPreferencesUsesTypedInput(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	response := doJSON(t, http.MethodPatch, server.URL+"/preferences", appservice.UserPreferencesInput{
		Locale:                      appservice.LocaleEnglishUnitedStates,
		TimeZone:                    "America/New_York",
		MessageNotificationsEnabled: true,
		WorkspaceTabsEnabled:        true,
	}, "test-token")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if backend.lastMeta.Token != "test-token" || backend.lastPreferences.Locale != appservice.LocaleEnglishUnitedStates || backend.lastPreferences.TimeZone != "America/New_York" || !backend.lastPreferences.MessageNotificationsEnabled || !backend.lastPreferences.WorkspaceTabsEnabled {
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

// TestGeneralSettingsUseTypedContract 验证通用设置接口保存类型化契约。
func TestGeneralSettingsUseTypedContract(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	updateResponse := doJSON(t, http.MethodPut, server.URL+"/settings/organization", appservice.OrganizationInput{
		Name: "鹿行协作", AllowArbitraryURL: true,
	}, "test-token")
	defer updateResponse.Body.Close()
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want %d", updateResponse.StatusCode, http.StatusOK)
	}
	var organization appservice.Organization
	if err := json.NewDecoder(updateResponse.Body).Decode(&organization); err != nil {
		t.Fatal(err)
	}
	if organization.Name != "鹿行协作" || !organization.AllowArbitraryURL || backend.lastMeta.Token != "test-token" || backend.lastOrganization.Name != "鹿行协作" || !backend.lastOrganization.AllowArbitraryURL {
		t.Fatalf("organization = %#v, meta = %#v", organization, backend.lastMeta)
	}
}

// TestBusinessSystemRoutesUseTypedContract 验证业务系统管理接口的方法、状态码和类型化输入。
func TestBusinessSystemRoutesUseTypedContract(t *testing.T) {
	backend := &testBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()

	listResponse := doJSON(t, http.MethodGet, server.URL+"/integrations/business-systems", nil, "test-token")
	defer listResponse.Body.Close()
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResponse.StatusCode, http.StatusOK)
	}

	createInput := appservice.BusinessSystemInput{Name: "企业 ERP", URL: "https://erp.example.com", Enabled: true}
	createResponse := doJSON(t, http.MethodPost, server.URL+"/integrations/business-systems", createInput, "test-token")
	defer createResponse.Body.Close()
	if createResponse.StatusCode != http.StatusCreated || backend.lastBusinessSystem != createInput {
		t.Fatalf("create status = %d, input = %#v", createResponse.StatusCode, backend.lastBusinessSystem)
	}

	getResponse := doJSON(t, http.MethodGet, server.URL+"/integrations/business-systems/business-system-1", nil, "test-token")
	defer getResponse.Body.Close()
	if getResponse.StatusCode != http.StatusOK || backend.lastBusinessSystemID != "business-system-1" {
		t.Fatalf("get status = %d, ID = %q", getResponse.StatusCode, backend.lastBusinessSystemID)
	}

	updateInput := appservice.BusinessSystemInput{Name: "企业 ERP", URL: "https://erp.example.com/workbench", Enabled: false}
	updateResponse := doJSON(t, http.MethodPut, server.URL+"/integrations/business-systems/business-system-1", updateInput, "test-token")
	defer updateResponse.Body.Close()
	if updateResponse.StatusCode != http.StatusOK || backend.lastBusinessSystemID != "business-system-1" || backend.lastBusinessSystem != updateInput {
		t.Fatalf("update status = %d, ID = %q, input = %#v", updateResponse.StatusCode, backend.lastBusinessSystemID, backend.lastBusinessSystem)
	}

	deleteResponse := doJSON(t, http.MethodDelete, server.URL+"/integrations/business-systems/business-system-1", nil, "test-token")
	defer deleteResponse.Body.Close()
	if deleteResponse.StatusCode != http.StatusNoContent || backend.lastBusinessSystemID != "business-system-1" {
		t.Fatalf("delete status = %d, ID = %q", deleteResponse.StatusCode, backend.lastBusinessSystemID)
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
		User:         appservice.CurrentUser{ID: "user-1", OrganizationID: "organization-1", Email: "admin@example.com", DisplayName: "管理员", RoleID: "role-1", Status: "active", Locale: "zh-CN", TimeZone: "Asia/Shanghai", MessageNotificationsEnabled: true, WorkStatus: appservice.WorkStatusWorking},
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
