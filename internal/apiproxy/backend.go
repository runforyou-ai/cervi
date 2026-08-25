//go:build !server

package apiproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

var (
	_ appservice.Backend         = (*Backend)(nil)
	_ appservice.ServerConnector = (*Backend)(nil)
)

// Backend 将类型化应用服务调用转换为远程 HTTP 请求。
type Backend struct {
	connection *connection
	sessions   *clientsession.Manager
	sessionMu  sync.Mutex
}

// NewBackend 创建原生端使用的远程应用后端。
func NewBackend(store Store, sessions *clientsession.Manager) (*Backend, error) {
	remoteConnection, err := newConnection(store)
	if err != nil {
		return nil, err
	}
	return &Backend{connection: remoteConnection, sessions: sessions}, nil
}

// InstallationStatus 不读取登录凭据并返回远程初始化状态。
func (b *Backend) InstallationStatus(ctx context.Context, meta appservice.RequestMeta) (appservice.InstallationStatus, error) {
	state := b.connection.currentState()
	if state == nil {
		return appservice.InstallationStatus{}, appservice.SessionError(meta, appservice.SessionStateConnect, cervii18n.ErrorServerConnectionRequired)
	}
	status, err := probeServer(ctx, state)
	if err == nil {
		return status, nil
	}
	if ctx.Err() != nil {
		return appservice.InstallationStatus{}, ctx.Err()
	}
	slog.Warn("检测已连接企业服务器失败", "server_url", state.baseURL.String(), "error", err)
	return appservice.InstallationStatus{}, appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
}

// Login 校验账号密码并建立原生端登录会话。
func (b *Backend) Login(ctx context.Context, meta appservice.RequestMeta, input appservice.LoginInput) (appservice.Auth, error) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	var output appservice.Auth
	if err := b.do(ctx, meta, http.MethodPost, "/auth/login", nil, input, &output); err != nil {
		return appservice.Auth{}, err
	}
	b.normalizeUser(&output.Identity.User)
	state := b.connection.currentState()
	if err := b.sessions.Establish(ctx, clientsession.Credential{
		ServerURL:      state.baseURL.String(),
		OrganizationID: output.Identity.Organization.ID,
		UserID:         output.Identity.User.ID,
		Token:          output.Token,
		ExpiresAt:      output.ExpiresAt,
	}); err != nil {
		slog.Warn("保存原生端登录凭据失败", "server_url", state.baseURL.String(), "user_id", output.Identity.User.ID, "error", err)
		return appservice.Auth{}, appservice.FailedError(meta, cervii18n.ErrorLoginFailed)
	}
	return appservice.Auth{Identity: output.Identity}, nil
}

// Logout 退出远程会话并清除原生端登录凭据。
func (b *Backend) Logout(ctx context.Context, meta appservice.RequestMeta) error {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	remoteErr := b.do(ctx, meta, http.MethodPost, "/auth/logout", nil, nil, nil)
	// 远程请求取消后仍清除本地凭据。
	if err := b.sessions.Clear(context.WithoutCancel(ctx)); err != nil {
		slog.Warn("清理原生端登录凭据失败", "error", err)
		return appservice.FailedError(meta, cervii18n.ErrorLogoutFailed)
	}
	return remoteErr
}

// LoadIdentity 返回当前远程登录身份。
func (b *Backend) LoadIdentity(ctx context.Context, meta appservice.RequestMeta) (appservice.Identity, error) {
	var output appservice.Identity
	err := b.do(ctx, meta, http.MethodGet, "/auth/identity", nil, nil, &output)
	b.normalizeUser(&output.User)
	return output, err
}

// UpdateProfile 修改远程当前用户的头像、姓名和邮箱。
func (b *Backend) UpdateProfile(ctx context.Context, meta appservice.RequestMeta, input appservice.ProfileInput) (appservice.CurrentUser, error) {
	var output appservice.CurrentUser
	err := b.do(ctx, meta, http.MethodPatch, "/profile", nil, input, &output)
	b.normalizeUser(&output)
	return output, err
}

// CreateFileUpload 创建远程文件上传请求。
func (b *Backend) CreateFileUpload(ctx context.Context, meta appservice.RequestMeta, input appservice.FileUploadInput) (appservice.FileUpload, error) {
	var output appservice.FileUpload
	err := b.do(ctx, meta, http.MethodPost, "/files/uploads", nil, input, &output)
	b.normalizeFile(&output.File)
	output.Request.URL = b.absoluteContentURL(output.Request.URL)
	return output, err
}

// CompleteFileUpload 核验并完成远程文件上传。
func (b *Backend) CompleteFileUpload(ctx context.Context, meta appservice.RequestMeta, fileID string) (appservice.File, error) {
	var output appservice.File
	err := b.do(ctx, meta, http.MethodPost, "/files/"+url.PathEscape(fileID)+"/complete", nil, nil, &output)
	b.normalizeFile(&output)
	return output, err
}

// ChangePassword 修改远程当前用户的登录密码。
func (b *Backend) ChangePassword(ctx context.Context, meta appservice.RequestMeta, input appservice.ChangePasswordInput) error {
	return b.do(ctx, meta, http.MethodPatch, "/password", nil, input, nil)
}

// UpdateUserPreferences 保存远程当前用户的偏好设置。
func (b *Backend) UpdateUserPreferences(ctx context.Context, meta appservice.RequestMeta, input appservice.UserPreferencesInput) (appservice.CurrentUser, error) {
	var output appservice.CurrentUser
	err := b.do(ctx, meta, http.MethodPatch, "/preferences", nil, input, &output)
	b.normalizeUser(&output)
	return output, err
}

// UpdateUserWorkStatus 保存远程当前用户主动设置的工作状态。
func (b *Backend) UpdateUserWorkStatus(ctx context.Context, meta appservice.RequestMeta, input appservice.UserWorkStatusInput) (appservice.CurrentUser, error) {
	var output appservice.CurrentUser
	err := b.do(ctx, meta, http.MethodPatch, "/work-status", nil, input, &output)
	b.normalizeUser(&output)
	return output, err
}

// LoadInbox 返回当前用户的远程收件箱。
func (b *Backend) LoadInbox(ctx context.Context, meta appservice.RequestMeta) (appservice.Inbox, error) {
	var output appservice.Inbox
	err := b.do(ctx, meta, http.MethodGet, "/inbox", nil, nil, &output)
	b.normalizeUser(&output.User)
	return output, err
}

// normalizeUser 将服务端相对头像地址转换为企业服务器绝对地址。
func (b *Backend) normalizeUser(user *appservice.CurrentUser) {
	user.AvatarURL = b.absoluteContentURL(user.AvatarURL)
}

// normalizeFile 将服务端相对文件地址转换为企业服务器绝对地址。
func (b *Backend) normalizeFile(file *appservice.File) {
	file.ContentURL = b.absoluteContentURL(file.ContentURL)
}

// absoluteContentURL 为原生端补全企业服务器文件地址。
func (b *Backend) absoluteContentURL(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	state := b.connection.currentState()
	if state == nil {
		return value
	}
	endpoint := *state.baseURL
	endpoint.Path = strings.TrimRight(state.baseURL.Path, "/") + "/" + strings.TrimLeft(value, "/")
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return endpoint.String()
}

// ListMessageChannels 返回远程消息渠道列表。
func (b *Backend) ListMessageChannels(ctx context.Context, meta appservice.RequestMeta) (appservice.MessageChannelList, error) {
	var output appservice.MessageChannelList
	err := b.do(ctx, meta, http.MethodGet, "/channels", nil, nil, &output)
	return output, err
}

// GetWebsiteChannel 返回远程网站渠道详情。
func (b *Backend) GetWebsiteChannel(ctx context.Context, meta appservice.RequestMeta, channelID string) (appservice.WebsiteChannel, error) {
	var output appservice.WebsiteChannel
	err := b.do(ctx, meta, http.MethodGet, "/channels/website/"+url.PathEscape(channelID), nil, nil, &output)
	return output, err
}

// GetMessageChannel 返回远程消息渠道基础信息。
func (b *Backend) GetMessageChannel(ctx context.Context, meta appservice.RequestMeta, channelID string) (appservice.MessageChannelSummary, error) {
	var output appservice.MessageChannelSummary
	err := b.do(ctx, meta, http.MethodGet, "/channels/"+url.PathEscape(channelID), nil, nil, &output)
	return output, err
}

// CreateMessageChannel 创建远程消息渠道。
func (b *Backend) CreateMessageChannel(ctx context.Context, meta appservice.RequestMeta, input appservice.CreateMessageChannelInput) (appservice.MessageChannelSummary, error) {
	var output appservice.MessageChannelSummary
	err := b.do(ctx, meta, http.MethodPost, "/channels", nil, input, &output)
	return output, err
}

// UpdateMessageChannel 修改远程消息渠道基础信息。
func (b *Backend) UpdateMessageChannel(ctx context.Context, meta appservice.RequestMeta, channelID string, input appservice.MessageChannelInput) (appservice.MessageChannelSummary, error) {
	var output appservice.MessageChannelSummary
	err := b.do(ctx, meta, http.MethodPatch, "/channels/"+url.PathEscape(channelID), nil, input, &output)
	return output, err
}

// UpdateWebsiteChannelChatInterface 修改远程网站渠道聊天界面。
func (b *Backend) UpdateWebsiteChannelChatInterface(ctx context.Context, meta appservice.RequestMeta, channelID string, input appservice.WebsiteChannelChatInterfaceInput) (appservice.WebsiteChannelChatInterface, error) {
	var output appservice.WebsiteChannelChatInterface
	err := b.do(ctx, meta, http.MethodPatch, "/channels/website/"+url.PathEscape(channelID)+"/chat-interface", nil, input, &output)
	return output, err
}

// UpdateWebsiteChannelAccess 修改远程网站渠道允许使用的网站。
func (b *Backend) UpdateWebsiteChannelAccess(ctx context.Context, meta appservice.RequestMeta, channelID string, input appservice.WebsiteChannelAccessInput) (appservice.WebsiteChannelAccess, error) {
	var output appservice.WebsiteChannelAccess
	err := b.do(ctx, meta, http.MethodPatch, "/channels/website/"+url.PathEscape(channelID)+"/access", nil, input, &output)
	return output, err
}

// DeactivateMessageChannel 停用远程消息渠道。
func (b *Backend) DeactivateMessageChannel(ctx context.Context, meta appservice.RequestMeta, channelID string) (appservice.MessageChannelSummary, error) {
	var output appservice.MessageChannelSummary
	err := b.do(ctx, meta, http.MethodPost, "/channels/"+url.PathEscape(channelID)+"/deactivate", nil, nil, &output)
	return output, err
}

// ActivateMessageChannel 启用远程消息渠道。
func (b *Backend) ActivateMessageChannel(ctx context.Context, meta appservice.RequestMeta, channelID string) (appservice.MessageChannelSummary, error) {
	var output appservice.MessageChannelSummary
	err := b.do(ctx, meta, http.MethodPost, "/channels/"+url.PathEscape(channelID)+"/activate", nil, nil, &output)
	return output, err
}

// ListChannelOptions 返回远程渠道选择项。
func (b *Backend) ListChannelOptions(ctx context.Context, meta appservice.RequestMeta) (appservice.ChannelOptionList, error) {
	var output appservice.ChannelOptionList
	err := b.do(ctx, meta, http.MethodGet, "/channel-options", nil, nil, &output)
	return output, err
}

// ListMemberOptions 返回远程企业成员选择项。
func (b *Backend) ListMemberOptions(ctx context.Context, meta appservice.RequestMeta, input appservice.MemberOptionListInput) (appservice.MemberOptionList, error) {
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.MemberOptionList
	err := b.do(ctx, meta, http.MethodGet, "/members/options", query, nil, &output)
	for index := range output.Members {
		output.Members[index].AvatarURL = b.absoluteContentURL(output.Members[index].AvatarURL)
	}
	return output, err
}

// CreateAgent 在远程企业服务器创建 AI 员工。
func (b *Backend) CreateAgent(ctx context.Context, meta appservice.RequestMeta, input appservice.CreateAgentInput) (appservice.Agent, error) {
	var output appservice.Agent
	err := b.do(ctx, meta, http.MethodPost, "/agents", nil, input, &output)
	return output, err
}

// ListAgentModelOptions 返回远程企业 AI 员工可使用的对话模型。
func (b *Backend) ListAgentModelOptions(ctx context.Context, meta appservice.RequestMeta) (appservice.AgentModelOptionList, error) {
	var output appservice.AgentModelOptionList
	err := b.do(ctx, meta, http.MethodGet, "/agents/model-options", nil, nil, &output)
	return output, err
}

// ListAgents 返回远程企业 AI 员工目录。
func (b *Backend) ListAgents(ctx context.Context, meta appservice.RequestMeta, input appservice.AgentListInput) (appservice.AgentList, error) {
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setOptionalQuery(query, "status", input.Status)
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.AgentList
	err := b.do(ctx, meta, http.MethodGet, "/agents", query, nil, &output)
	return output, err
}

// GetAgent 返回远程企业 AI 员工详情。
func (b *Backend) GetAgent(ctx context.Context, meta appservice.RequestMeta, agentID string) (appservice.Agent, error) {
	var output appservice.Agent
	err := b.do(ctx, meta, http.MethodGet, "/agents/"+url.PathEscape(agentID), nil, nil, &output)
	return output, err
}

// UpdateAgent 修改远程企业 AI 员工。
func (b *Backend) UpdateAgent(ctx context.Context, meta appservice.RequestMeta, agentID string, input appservice.UpdateAgentInput) (appservice.Agent, error) {
	var output appservice.Agent
	err := b.do(ctx, meta, http.MethodPatch, "/agents/"+url.PathEscape(agentID), nil, input, &output)
	return output, err
}

// UpdateAgentExecution 修改远程企业 AI 员工执行配置。
func (b *Backend) UpdateAgentExecution(ctx context.Context, meta appservice.RequestMeta, agentID string, input appservice.AgentExecutionInput) (appservice.Agent, error) {
	var output appservice.Agent
	err := b.do(ctx, meta, http.MethodPatch, "/agents/"+url.PathEscape(agentID)+"/execution", nil, input, &output)
	return output, err
}

// UpdateAgentWorkStatus 修改远程企业 AI 员工工作状态。
func (b *Backend) UpdateAgentWorkStatus(ctx context.Context, meta appservice.RequestMeta, agentID string, input appservice.AgentWorkStatusInput) (appservice.Agent, error) {
	var output appservice.Agent
	err := b.do(ctx, meta, http.MethodPatch, "/agents/"+url.PathEscape(agentID)+"/work-status", nil, input, &output)
	return output, err
}

// DeactivateAgent 禁用远程企业 AI 员工账号。
func (b *Backend) DeactivateAgent(ctx context.Context, meta appservice.RequestMeta, agentID string) (appservice.Agent, error) {
	var output appservice.Agent
	err := b.do(ctx, meta, http.MethodPost, "/agents/"+url.PathEscape(agentID)+"/deactivate", nil, nil, &output)
	return output, err
}

// ReactivateAgent 恢复远程企业 AI 员工。
func (b *Backend) ReactivateAgent(ctx context.Context, meta appservice.RequestMeta, agentID string) (appservice.Agent, error) {
	var output appservice.Agent
	err := b.do(ctx, meta, http.MethodPost, "/agents/"+url.PathEscape(agentID)+"/reactivate", nil, nil, &output)
	return output, err
}

// ListUsers 返回远程企业成员列表。
func (b *Backend) ListUsers(ctx context.Context, meta appservice.RequestMeta, input appservice.UserListInput) (appservice.UserList, error) {
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setOptionalQuery(query, "status", input.Status)
	setQuery(query, "roleId", input.RoleID)
	setQuery(query, "teamId", input.TeamID)
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.UserList
	err := b.do(ctx, meta, http.MethodGet, "/users", query, nil, &output)
	return output, err
}

// GetUser 返回远程企业成员详情。
func (b *Backend) GetUser(ctx context.Context, meta appservice.RequestMeta, userID string) (appservice.User, error) {
	var output appservice.User
	err := b.do(ctx, meta, http.MethodGet, "/users/"+url.PathEscape(userID), nil, nil, &output)
	return output, err
}

// CreateUser 创建远程企业成员账号。
func (b *Backend) CreateUser(ctx context.Context, meta appservice.RequestMeta, input appservice.CreateUserInput) (appservice.User, error) {
	var output appservice.User
	err := b.do(ctx, meta, http.MethodPost, "/users", nil, input, &output)
	return output, err
}

// UpdateUser 修改远程企业成员。
func (b *Backend) UpdateUser(ctx context.Context, meta appservice.RequestMeta, userID string, input appservice.UpdateUserInput) (appservice.User, error) {
	var output appservice.User
	err := b.do(ctx, meta, http.MethodPatch, "/users/"+url.PathEscape(userID), nil, input, &output)
	return output, err
}

// UpdateUserRoles 在远程企业服务器中批量调整成员角色。
func (b *Backend) UpdateUserRoles(ctx context.Context, meta appservice.RequestMeta, input appservice.UserRoleChangesInput) error {
	return b.do(ctx, meta, http.MethodPatch, "/users/roles", nil, input, nil)
}

// DeactivateUser 禁用远程企业成员账号。
func (b *Backend) DeactivateUser(ctx context.Context, meta appservice.RequestMeta, userID string) (appservice.User, error) {
	var output appservice.User
	err := b.do(ctx, meta, http.MethodPost, "/users/"+url.PathEscape(userID)+"/deactivate", nil, nil, &output)
	return output, err
}

// ReactivateUser 恢复远程企业成员账号。
func (b *Backend) ReactivateUser(ctx context.Context, meta appservice.RequestMeta, userID string) (appservice.User, error) {
	var output appservice.User
	err := b.do(ctx, meta, http.MethodPost, "/users/"+url.PathEscape(userID)+"/reactivate", nil, nil, &output)
	return output, err
}

// ListTeams 返回远程企业团队列表。
func (b *Backend) ListTeams(ctx context.Context, meta appservice.RequestMeta, input appservice.TeamListInput) (appservice.TeamList, error) {
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.TeamList
	err := b.do(ctx, meta, http.MethodGet, "/teams", query, nil, &output)
	return output, err
}

// CreateTeam 创建远程企业团队。
func (b *Backend) CreateTeam(ctx context.Context, meta appservice.RequestMeta, input appservice.TeamInput) (appservice.Team, error) {
	var output appservice.Team
	err := b.do(ctx, meta, http.MethodPost, "/teams", nil, input, &output)
	return output, err
}

// UpdateTeam 修改远程企业团队。
func (b *Backend) UpdateTeam(ctx context.Context, meta appservice.RequestMeta, teamID string, input appservice.TeamInput) (appservice.Team, error) {
	var output appservice.Team
	err := b.do(ctx, meta, http.MethodPatch, "/teams/"+url.PathEscape(teamID), nil, input, &output)
	return output, err
}

// DeleteTeam 删除远程企业团队。
func (b *Backend) DeleteTeam(ctx context.Context, meta appservice.RequestMeta, teamID string) error {
	return b.do(ctx, meta, http.MethodDelete, "/teams/"+url.PathEscape(teamID), nil, nil, nil)
}

// ListKnowledgeBases 返回远程企业知识库列表。
func (b *Backend) ListKnowledgeBases(ctx context.Context, meta appservice.RequestMeta) (appservice.KnowledgeBaseList, error) {
	var output appservice.KnowledgeBaseList
	err := b.do(ctx, meta, http.MethodGet, "/knowledge-bases", nil, nil, &output)
	return output, err
}

// GetKnowledgeBase 返回远程企业知识库详情。
func (b *Backend) GetKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, knowledgeBaseID string) (appservice.KnowledgeBase, error) {
	var output appservice.KnowledgeBase
	err := b.do(ctx, meta, http.MethodGet, knowledgeBasePath(knowledgeBaseID), nil, nil, &output)
	return output, err
}

// CreateKnowledgeBase 创建远程企业知识库。
func (b *Backend) CreateKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, input appservice.KnowledgeBaseInput) (appservice.KnowledgeBase, error) {
	var output appservice.KnowledgeBase
	err := b.do(ctx, meta, http.MethodPost, "/knowledge-bases", nil, input, &output)
	return output, err
}

// UpdateKnowledgeBase 修改远程企业知识库。
func (b *Backend) UpdateKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, knowledgeBaseID string, input appservice.KnowledgeBaseInput) (appservice.KnowledgeBase, error) {
	var output appservice.KnowledgeBase
	err := b.do(ctx, meta, http.MethodPatch, knowledgeBasePath(knowledgeBaseID), nil, input, &output)
	return output, err
}

// DeleteKnowledgeBase 删除远程企业知识库。
func (b *Backend) DeleteKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, knowledgeBaseID string) error {
	return b.do(ctx, meta, http.MethodDelete, knowledgeBasePath(knowledgeBaseID), nil, nil, nil)
}

// CreateKnowledgeGroup 创建远程知识库分组。
func (b *Backend) CreateKnowledgeGroup(ctx context.Context, meta appservice.RequestMeta, knowledgeBaseID string, input appservice.KnowledgeGroupInput) (appservice.KnowledgeBase, error) {
	var output appservice.KnowledgeBase
	err := b.do(ctx, meta, http.MethodPost, knowledgeBasePath(knowledgeBaseID)+"/groups", nil, input, &output)
	return output, err
}

// UpdateKnowledgeGroup 修改远程知识库分组。
func (b *Backend) UpdateKnowledgeGroup(ctx context.Context, meta appservice.RequestMeta, knowledgeBaseID, groupID string, input appservice.KnowledgeGroupInput) (appservice.KnowledgeBase, error) {
	var output appservice.KnowledgeBase
	err := b.do(ctx, meta, http.MethodPatch, knowledgeBasePath(knowledgeBaseID)+"/groups/"+url.PathEscape(groupID), nil, input, &output)
	return output, err
}

// DeleteKnowledgeGroup 删除远程知识库分组。
func (b *Backend) DeleteKnowledgeGroup(ctx context.Context, meta appservice.RequestMeta, knowledgeBaseID, groupID string) (appservice.KnowledgeBase, error) {
	var output appservice.KnowledgeBase
	err := b.do(ctx, meta, http.MethodDelete, knowledgeBasePath(knowledgeBaseID)+"/groups/"+url.PathEscape(groupID), nil, nil, &output)
	return output, err
}

// knowledgeBasePath 返回远程知识库路径。
func knowledgeBasePath(knowledgeBaseID string) string {
	return "/knowledge-bases/" + url.PathEscape(knowledgeBaseID)
}

// ListTeamMembers 返回远程团队成员列表。
func (b *Backend) ListTeamMembers(ctx context.Context, meta appservice.RequestMeta, teamID string, input appservice.TeamMemberListInput) (appservice.TeamMemberList, error) {
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setOptionalQuery(query, "workStatus", input.WorkStatus)
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.TeamMemberList
	err := b.do(ctx, meta, http.MethodGet, "/teams/"+url.PathEscape(teamID)+"/members", query, nil, &output)
	return output, err
}

// ListTeamMemberCandidates 返回远程团队可添加的企业身份。
func (b *Backend) ListTeamMemberCandidates(ctx context.Context, meta appservice.RequestMeta, teamID string, input appservice.TeamMemberCandidateInput) (appservice.TeamMemberCandidateList, error) {
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.TeamMemberCandidateList
	err := b.do(ctx, meta, http.MethodGet, "/teams/"+url.PathEscape(teamID)+"/member-candidates", query, nil, &output)
	for index := range output.Members {
		output.Members[index].AvatarURL = b.absoluteContentURL(output.Members[index].AvatarURL)
	}
	return output, err
}

// AddTeamMembers 将远程企业身份批量加入团队。
func (b *Backend) AddTeamMembers(ctx context.Context, meta appservice.RequestMeta, teamID string, input appservice.TeamMemberInput) (appservice.Team, error) {
	var output appservice.Team
	err := b.do(ctx, meta, http.MethodPost, "/teams/"+url.PathEscape(teamID)+"/members", nil, input, &output)
	return output, err
}

// RemoveTeamMembers 将远程企业身份批量移出团队。
func (b *Backend) RemoveTeamMembers(ctx context.Context, meta appservice.RequestMeta, teamID string, input appservice.TeamMemberInput) (appservice.Team, error) {
	var output appservice.Team
	err := b.do(ctx, meta, http.MethodDelete, "/teams/"+url.PathEscape(teamID)+"/members", nil, input, &output)
	return output, err
}

// ListContacts 返回远程联系人列表。
func (b *Backend) ListContacts(ctx context.Context, meta appservice.RequestMeta, input appservice.ContactListInput) (appservice.ContactList, error) {
	path := "/contacts"
	if input.Deleted {
		path += "/trash"
	}
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setOptionalQuery(query, "stage", input.Stage)
	setQuery(query, "channelId", input.ChannelID)
	setOptionalQuery(query, "methodType", input.MethodType)
	setQuery(query, "sort", string(input.Sort))
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.ContactList
	err := b.do(ctx, meta, http.MethodGet, path, query, nil, &output)
	return output, err
}

// GetContact 返回远程联系人详情。
func (b *Backend) GetContact(ctx context.Context, meta appservice.RequestMeta, contactID string) (appservice.Contact, error) {
	var output appservice.Contact
	err := b.do(ctx, meta, http.MethodGet, "/contacts/"+url.PathEscape(contactID), nil, nil, &output)
	return output, err
}

// CreateContact 创建远程联系人。
func (b *Backend) CreateContact(ctx context.Context, meta appservice.RequestMeta, input appservice.ContactInput) (appservice.Contact, error) {
	var output appservice.Contact
	err := b.do(ctx, meta, http.MethodPost, "/contacts", nil, input, &output)
	return output, err
}

// UpdateContact 修改远程联系人。
func (b *Backend) UpdateContact(ctx context.Context, meta appservice.RequestMeta, contactID string, input appservice.ContactInput) (appservice.Contact, error) {
	var output appservice.Contact
	err := b.do(ctx, meta, http.MethodPatch, "/contacts/"+url.PathEscape(contactID), nil, input, &output)
	return output, err
}

// DeleteContact 将远程联系人移入回收站。
func (b *Backend) DeleteContact(ctx context.Context, meta appservice.RequestMeta, contactID string) error {
	return b.do(ctx, meta, http.MethodDelete, "/contacts/"+url.PathEscape(contactID), nil, nil, nil)
}

// RestoreContact 恢复远程联系人。
func (b *Backend) RestoreContact(ctx context.Context, meta appservice.RequestMeta, contactID string) (appservice.Contact, error) {
	var output appservice.Contact
	err := b.do(ctx, meta, http.MethodPost, "/contacts/"+url.PathEscape(contactID)+"/restore", nil, nil, &output)
	return output, err
}

// ListRoles 返回远程企业角色和预定义权限目录。
func (b *Backend) ListRoles(ctx context.Context, meta appservice.RequestMeta) (appservice.RoleList, error) {
	var output appservice.RoleList
	err := b.do(ctx, meta, http.MethodGet, "/settings/roles", nil, nil, &output)
	return output, err
}

// GetRole 返回远程企业角色详情。
func (b *Backend) GetRole(ctx context.Context, meta appservice.RequestMeta, roleID string) (appservice.Role, error) {
	var output appservice.Role
	err := b.do(ctx, meta, http.MethodGet, "/settings/roles/"+url.PathEscape(roleID), nil, nil, &output)
	return output, err
}

// CreateRole 创建远程企业自定义角色。
func (b *Backend) CreateRole(ctx context.Context, meta appservice.RequestMeta, input appservice.RoleInput) (appservice.Role, error) {
	var output appservice.Role
	err := b.do(ctx, meta, http.MethodPost, "/settings/roles", nil, input, &output)
	return output, err
}

// UpdateRole 修改远程企业角色。
func (b *Backend) UpdateRole(ctx context.Context, meta appservice.RequestMeta, roleID string, input appservice.RoleInput) (appservice.Role, error) {
	var output appservice.Role
	err := b.do(ctx, meta, http.MethodPut, "/settings/roles/"+url.PathEscape(roleID), nil, input, &output)
	return output, err
}

// DeleteRole 删除远程企业自定义角色。
func (b *Backend) DeleteRole(ctx context.Context, meta appservice.RequestMeta, roleID string) error {
	return b.do(ctx, meta, http.MethodDelete, "/settings/roles/"+url.PathEscape(roleID), nil, nil, nil)
}

// ListAIProviders 返回远程模型服务供应商列表。
func (b *Backend) ListAIProviders(ctx context.Context, meta appservice.RequestMeta) (appservice.AIProviderList, error) {
	var output appservice.AIProviderList
	err := b.do(ctx, meta, http.MethodGet, "/integrations/model-services", nil, nil, &output)
	return output, err
}

// GetAIProvider 返回远程模型服务供应商详情。
func (b *Backend) GetAIProvider(ctx context.Context, meta appservice.RequestMeta, providerID string) (appservice.AIProvider, error) {
	var output appservice.AIProvider
	err := b.do(ctx, meta, http.MethodGet, "/integrations/model-services/"+url.PathEscape(providerID), nil, nil, &output)
	return output, err
}

// ListAvailableAIModels 返回远程指定品牌的预设模型目录。
func (b *Backend) ListAvailableAIModels(ctx context.Context, meta appservice.RequestMeta, brand appservice.AIProviderBrand) (appservice.AIProviderModelList, error) {
	query := url.Values{}
	query.Set("brand", string(brand))
	var output appservice.AIProviderModelList
	err := b.do(ctx, meta, http.MethodGet, "/integrations/model-services/models", query, nil, &output)
	return output, err
}

// CreateAIProvider 创建远程模型服务供应商。
func (b *Backend) CreateAIProvider(ctx context.Context, meta appservice.RequestMeta, input appservice.AIProviderInput) (appservice.AIProvider, error) {
	var output appservice.AIProvider
	err := b.do(ctx, meta, http.MethodPost, "/integrations/model-services", nil, input, &output)
	return output, err
}

// UpdateAIProvider 修改远程模型服务供应商。
func (b *Backend) UpdateAIProvider(ctx context.Context, meta appservice.RequestMeta, providerID string, input appservice.AIProviderInput) (appservice.AIProvider, error) {
	var output appservice.AIProvider
	err := b.do(ctx, meta, http.MethodPut, "/integrations/model-services/"+url.PathEscape(providerID), nil, input, &output)
	return output, err
}

// DeleteAIProvider 删除远程模型服务供应商。
func (b *Backend) DeleteAIProvider(ctx context.Context, meta appservice.RequestMeta, providerID string) error {
	return b.do(ctx, meta, http.MethodDelete, "/integrations/model-services/"+url.PathEscape(providerID), nil, nil, nil)
}

// UpdateOrganization 修改远程企业名称。
func (b *Backend) UpdateOrganization(ctx context.Context, meta appservice.RequestMeta, input appservice.OrganizationInput) (appservice.Organization, error) {
	var output appservice.Organization
	err := b.do(ctx, meta, http.MethodPut, "/settings/organization", nil, input, &output)
	return output, err
}

// GetS3Setting 返回远程对象存储设置。
func (b *Backend) GetS3Setting(ctx context.Context, meta appservice.RequestMeta) (appservice.S3Setting, error) {
	var output appservice.S3Setting
	err := b.do(ctx, meta, http.MethodGet, "/settings/storage/s3", nil, nil, &output)
	return output, err
}

// SaveS3Setting 保存远程对象存储设置。
func (b *Backend) SaveS3Setting(ctx context.Context, meta appservice.RequestMeta, input appservice.S3Setting) (appservice.S3Setting, error) {
	var output appservice.S3Setting
	err := b.do(ctx, meta, http.MethodPut, "/settings/storage/s3", nil, input, &output)
	return output, err
}

// TestS3Setting 测试远程对象存储连接。
func (b *Backend) TestS3Setting(ctx context.Context, meta appservice.RequestMeta, input appservice.S3Setting) error {
	return b.do(ctx, meta, http.MethodPost, "/settings/storage/s3/test", nil, input, nil)
}

// ServerURL 返回当前配置的企业服务器地址。
func (b *Backend) ServerURL(_ context.Context, _ appservice.RequestMeta) (string, error) {
	state := b.connection.currentState()
	if state == nil {
		return "", nil
	}
	return state.baseURL.String(), nil
}

// ProbeServer 检测企业服务器并返回公开企业名称，不保存地址。
func (b *Backend) ProbeServer(ctx context.Context, meta appservice.RequestMeta, serverURL string) (appservice.InstallationStatus, error) {
	state, status, err := b.inspectServer(ctx, meta, serverURL)
	if err != nil {
		return appservice.InstallationStatus{}, err
	}
	slog.Info("已检测到企业服务器", "server_url", state.baseURL.String(), "organization", status.OrganizationName)
	return status, nil
}

// ConnectServer 验证并保存企业服务器地址。
func (b *Backend) ConnectServer(ctx context.Context, meta appservice.RequestMeta, serverURL string) error {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	state, _, err := b.inspectServer(ctx, meta, serverURL)
	if err != nil {
		return err
	}
	current := b.connection.currentState()
	changed := current == nil || current.baseURL.String() != state.baseURL.String()
	if changed {
		if err := b.sessions.Clear(ctx); err != nil {
			slog.Warn("切换企业服务器前清理登录凭据失败", "server_url", state.baseURL.String(), "error", err)
			return appservice.FailedError(meta, cervii18n.ErrorServerConnectionSaveFailed)
		}
	}
	if err := b.connection.store.SetServerURL(ctx, state.baseURL.String()); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("保存企业服务器配置失败", "server_url", state.baseURL.String(), "error", err)
		return appservice.FailedError(meta, cervii18n.ErrorServerConnectionSaveFailed)
	}
	b.connection.mu.Lock()
	b.connection.state = state
	b.connection.mu.Unlock()
	slog.Info("企业服务器连接成功", "server_url", state.baseURL.String(), "changed", changed)
	return nil
}

// inspectServer 校验地址并读取远程初始化状态，不保存配置。
func (b *Backend) inspectServer(ctx context.Context, meta appservice.RequestMeta, serverURL string) (*remoteState, appservice.InstallationStatus, error) {
	parsed, err := parseServerURL(serverURL)
	if err != nil {
		var validationError *serverURLValidationError
		if !errors.As(err, &validationError) {
			return nil, appservice.InstallationStatus{}, fmt.Errorf("parse enterprise server URL: %w", err)
		}
		return nil, appservice.InstallationStatus{}, appservice.InvalidError(meta, cervii18n.ErrorServerURLInvalid, map[string]cervii18n.Key{"serverUrl": validationError.messageKey})
	}
	state := newRemoteState(parsed)
	status, err := probeServer(ctx, state)
	if err != nil {
		if ctx.Err() != nil {
			return nil, appservice.InstallationStatus{}, ctx.Err()
		}
		slog.Warn("验证企业服务器失败", "server_url", parsed.String(), "error", err)
		return nil, appservice.InstallationStatus{}, appservice.UnavailableError(meta, cervii18n.ErrorServerUnavailable, map[string]cervii18n.Key{"serverUrl": cervii18n.FieldServerURLNotCervi})
	}
	if !status.Installed || status.OrganizationName == "" {
		slog.Info("企业服务器尚未初始化", "server_url", parsed.String())
		return nil, appservice.InstallationStatus{}, appservice.InvalidError(meta, cervii18n.ErrorServerInitializationRequired, nil)
	}
	return state, status, nil
}

// do 向已连接的企业服务器发送 HTTP 请求。
func (b *Backend) do(ctx context.Context, meta appservice.RequestMeta, method, path string, query url.Values, input, output any) error {
	state := b.connection.currentState()
	if state == nil {
		return appservice.SessionError(meta, appservice.SessionStateConnect, cervii18n.ErrorServerConnectionRequired)
	}
	credential, authenticated := b.sessions.Current(ctx, state.baseURL.String())
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode remote request: %w", err)
		}
		body = bytes.NewReader(payload)
	}
	rawQuery := ""
	if query != nil {
		rawQuery = query.Encode()
	}
	endpoint := remoteEndpoint(state.baseURL, path, rawQuery)
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return appservice.FailedError(meta, cervii18n.ErrorRemoteRequestCreateFailed)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", string(meta.Locale))
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		request.Header.Set("Authorization", "Bearer "+credential.Token)
	}
	response, err := state.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("企业服务器请求失败", "server_url", state.baseURL.String(), "method", method, "path", path, "error", err)
		return appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxResponseBytes)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var payload errorBody
		if err := json.NewDecoder(limited).Decode(&payload); err != nil {
			slog.Warn("解析企业服务器错误响应失败", "server_url", state.baseURL.String(), "method", method, "path", path, "status", response.StatusCode, "error", err)
			return &appservice.Error{Kind: appservice.ErrorKindFailed, Message: http.StatusText(response.StatusCode)}
		}
		sessionState := payload.Error.State
		if sessionState == appservice.SessionStateSetup {
			slog.Info("远端要求初始化，改为连接企业服务器")
			sessionState = appservice.SessionStateConnect
		}
		if sessionState == appservice.SessionStateLogin && authenticated {
			if err := b.sessions.ClearIfCurrent(ctx, credential); err != nil {
				slog.Warn("登录凭据失效后清理本地会话失败", "server_url", state.baseURL.String(), "error", err)
			}
		}
		return &appservice.Error{Kind: payload.Error.Kind, State: sessionState, Message: payload.Error.Message, Fields: payload.Error.Fields}
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(limited).Decode(output); err != nil {
		slog.Warn("解析企业服务器响应失败", "server_url", state.baseURL.String(), "method", method, "path", path, "status", response.StatusCode, "error", err)
		return appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	return nil
}

// setQuery 在值非空时写入查询参数。
func setQuery(query url.Values, name, value string) {
	if value != "" {
		query.Set(name, value)
	}
}

// setOptionalQuery 在指针非空时写入查询参数。
func setOptionalQuery[T ~string](query url.Values, name string, value *T) {
	if value != nil {
		setQuery(query, name, string(*value))
	}
}

// setPositiveQuery 在值为正数时写入查询参数。
func setPositiveQuery(query url.Values, name string, value int) {
	if value > 0 {
		query.Set(name, strconv.Itoa(value))
	}
}
