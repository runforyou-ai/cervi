package appservice

import "context"

// Backend 定义各运行平台都需要实现的业务调用。
type Backend interface {
	InstallationStatus(context.Context, RequestMeta) (InstallationStatus, error)
	Login(context.Context, RequestMeta, LoginInput) (Auth, error)
	Logout(context.Context, RequestMeta) error
	LoadIdentity(context.Context, RequestMeta) (Identity, error)
	UpdateProfile(context.Context, RequestMeta, ProfileInput) (CurrentUser, error)
	CreateFileUpload(context.Context, RequestMeta, FileUploadInput) (FileUpload, error)
	CompleteFileUpload(context.Context, RequestMeta, string) (File, error)
	ChangePassword(context.Context, RequestMeta, ChangePasswordInput) error
	UpdateUserPreferences(context.Context, RequestMeta, UserPreferencesInput) (CurrentUser, error)
	UpdateUserWorkStatus(context.Context, RequestMeta, UserWorkStatusInput) (CurrentUser, error)
	LoadInbox(context.Context, RequestMeta) (Inbox, error)
	ListMessageChannels(context.Context, RequestMeta) (MessageChannelList, error)
	GetWebsiteChannel(context.Context, RequestMeta, string) (WebsiteChannel, error)
	GetMessageChannel(context.Context, RequestMeta, string) (MessageChannelSummary, error)
	CreateMessageChannel(context.Context, RequestMeta, CreateMessageChannelInput) (MessageChannelSummary, error)
	UpdateMessageChannel(context.Context, RequestMeta, string, MessageChannelInput) (MessageChannelSummary, error)
	UpdateWebsiteChannelChatInterface(context.Context, RequestMeta, string, WebsiteChannelChatInterfaceInput) (WebsiteChannelChatInterface, error)
	UpdateWebsiteChannelAccess(context.Context, RequestMeta, string, WebsiteChannelAccessInput) (WebsiteChannelAccess, error)
	DeactivateMessageChannel(context.Context, RequestMeta, string) (MessageChannelSummary, error)
	ActivateMessageChannel(context.Context, RequestMeta, string) (MessageChannelSummary, error)
	ListChannelOptions(context.Context, RequestMeta) (ChannelOptionList, error)
	ListMemberOptions(context.Context, RequestMeta, MemberOptionListInput) (MemberOptionList, error)
	ListAgentModelOptions(context.Context, RequestMeta) (AgentModelOptionList, error)
	CreateAgent(context.Context, RequestMeta, CreateAgentInput) (Agent, error)
	ListAgents(context.Context, RequestMeta, AgentListInput) (AgentList, error)
	GetAgent(context.Context, RequestMeta, string) (Agent, error)
	UpdateAgent(context.Context, RequestMeta, string, UpdateAgentInput) (Agent, error)
	UpdateAgentCapability(context.Context, RequestMeta, string, AgentCapabilityInput) (Agent, error)
	UpdateAgentWorkStatus(context.Context, RequestMeta, string, AgentWorkStatusInput) (Agent, error)
	DeactivateAgent(context.Context, RequestMeta, string) (Agent, error)
	ReactivateAgent(context.Context, RequestMeta, string) (Agent, error)
	ListUsers(context.Context, RequestMeta, UserListInput) (UserList, error)
	GetUser(context.Context, RequestMeta, string) (User, error)
	CreateUser(context.Context, RequestMeta, CreateUserInput) (User, error)
	UpdateUser(context.Context, RequestMeta, string, UpdateUserInput) (User, error)
	UpdateUserRoles(context.Context, RequestMeta, UserRoleChangesInput) error
	DeactivateUser(context.Context, RequestMeta, string) (User, error)
	ReactivateUser(context.Context, RequestMeta, string) (User, error)
	ListTeams(context.Context, RequestMeta, TeamListInput) (TeamList, error)
	CreateTeam(context.Context, RequestMeta, TeamInput) (Team, error)
	UpdateTeam(context.Context, RequestMeta, string, TeamInput) (Team, error)
	DeleteTeam(context.Context, RequestMeta, string) error
	ListKnowledgeBases(context.Context, RequestMeta) (KnowledgeBaseList, error)
	GetKnowledgeBase(context.Context, RequestMeta, string) (KnowledgeBase, error)
	CreateKnowledgeBase(context.Context, RequestMeta, KnowledgeBaseInput) (KnowledgeBase, error)
	UpdateKnowledgeBase(context.Context, RequestMeta, string, KnowledgeBaseInput) (KnowledgeBase, error)
	DeleteKnowledgeBase(context.Context, RequestMeta, string) error
	CreateKnowledgeGroup(context.Context, RequestMeta, string, KnowledgeGroupInput) (KnowledgeBase, error)
	UpdateKnowledgeGroup(context.Context, RequestMeta, string, string, KnowledgeGroupInput) (KnowledgeBase, error)
	DeleteKnowledgeGroup(context.Context, RequestMeta, string, string) (KnowledgeBase, error)
	ListTeamMembers(context.Context, RequestMeta, string, TeamMemberListInput) (TeamMemberList, error)
	ListTeamMemberCandidates(context.Context, RequestMeta, string, TeamMemberCandidateInput) (TeamMemberCandidateList, error)
	AddTeamMembers(context.Context, RequestMeta, string, TeamMemberInput) (Team, error)
	RemoveTeamMembers(context.Context, RequestMeta, string, TeamMemberInput) (Team, error)
	ListContacts(context.Context, RequestMeta, ContactListInput) (ContactList, error)
	GetContact(context.Context, RequestMeta, string) (Contact, error)
	CreateContact(context.Context, RequestMeta, ContactInput) (Contact, error)
	UpdateContact(context.Context, RequestMeta, string, ContactInput) (Contact, error)
	DeleteContact(context.Context, RequestMeta, string) error
	RestoreContact(context.Context, RequestMeta, string) (Contact, error)
	ListRoles(context.Context, RequestMeta) (RoleList, error)
	GetRole(context.Context, RequestMeta, string) (Role, error)
	CreateRole(context.Context, RequestMeta, RoleInput) (Role, error)
	UpdateRole(context.Context, RequestMeta, string, RoleInput) (Role, error)
	DeleteRole(context.Context, RequestMeta, string) error
	ListAIProviders(context.Context, RequestMeta) (AIProviderList, error)
	GetAIProvider(context.Context, RequestMeta, string) (AIProvider, error)
	ListAvailableAIModels(context.Context, RequestMeta, AIProviderBrand) (AIProviderModelList, error)
	CreateAIProvider(context.Context, RequestMeta, AIProviderInput) (AIProvider, error)
	UpdateAIProvider(context.Context, RequestMeta, string, AIProviderInput) (AIProvider, error)
	DeleteAIProvider(context.Context, RequestMeta, string) error
	UpdateOrganization(context.Context, RequestMeta, OrganizationInput) (Organization, error)
	GetS3Setting(context.Context, RequestMeta) (S3Setting, error)
	SaveS3Setting(context.Context, RequestMeta, S3Setting) (S3Setting, error)
	TestS3Setting(context.Context, RequestMeta, S3Setting) error
}

// WorkspaceInstaller 由服务端 Backend 实现，用于企业初始化。
type WorkspaceInstaller interface {
	InstallWorkspace(context.Context, RequestMeta, InstallWorkspaceInput) (Auth, error)
}

// ServerConnector 由原生端 Backend 实现，用于企业服务器地址。
type ServerConnector interface {
	ServerURL(context.Context, RequestMeta) (string, error)
	ProbeServer(context.Context, RequestMeta, string) (InstallationStatus, error)
	ConnectServer(context.Context, RequestMeta, string) error
}

// ProfileImageSelector 由支持原生文件对话框的平台实现。
type ProfileImageSelector interface {
	SelectProfileImage(context.Context, RequestMeta) (ProfileImageFile, error)
}

// NativeLocaleUpdater 同步当前设备上的原生界面语言。
type NativeLocaleUpdater interface {
	SetLocale(Locale)
}

// NativeNotification 由原生端实现系统通知权限和消息投递。
type NativeNotification interface {
	CheckNotificationPermission(context.Context, RequestMeta) (NotificationPermissionStatus, error)
	RequestNotificationPermission(context.Context, RequestMeta) (NotificationPermissionStatus, error)
	SendMessageNotification(context.Context, RequestMeta, MessageNotificationInput) error
}

// Service 将跨平台业务调用转发给当前运行平台的 Backend。
type Service struct {
	backend              Backend
	profileImageSelector ProfileImageSelector
	nativeLocaleUpdater  NativeLocaleUpdater
	nativeNotification   NativeNotification
	unreadIndicator      UnreadIndicator
}

// Option 配置平台专属的应用服务能力。
type Option func(*Service)

// WithProfileImageSelector 注入原生端头像文件选择器。
func WithProfileImageSelector(selector ProfileImageSelector) Option {
	return func(service *Service) {
		service.profileImageSelector = selector
	}
}

// WithNativeLocaleUpdater 注入原生界面语言同步能力。
func WithNativeLocaleUpdater(updater NativeLocaleUpdater) Option {
	return func(service *Service) {
		service.nativeLocaleUpdater = updater
	}
}

// WithNativeNotification 注入原生端系统通知能力。
func WithNativeNotification(notification NativeNotification) Option {
	return func(service *Service) {
		service.nativeNotification = notification
	}
}

// WithUnreadIndicator 注入原生端未读提示能力。
func WithUnreadIndicator(indicator UnreadIndicator) Option {
	return func(service *Service) {
		service.unreadIndicator = indicator
	}
}

// New 创建跨平台应用服务。
func New(backend Backend, options ...Option) *Service {
	service := &Service{backend: backend}
	for _, option := range options {
		option(service)
	}
	return service
}

// InstallationStatus 返回服务端初始化状态和公开企业名称。
func (s *Service) InstallationStatus(ctx context.Context, meta RequestMeta) (InstallationStatus, error) {
	return s.backend.InstallationStatus(ctx, meta)
}

// InstallWorkspace 创建企业管理员并返回登录令牌。
func (s *Service) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	installer, ok := s.backend.(WorkspaceInstaller)
	if !ok {
		return Auth{}, methodNotAllowedError(meta, "InstallWorkspace")
	}
	return installer.InstallWorkspace(ctx, meta, input)
}

// Login 校验账号密码并建立登录会话。
func (s *Service) Login(ctx context.Context, meta RequestMeta, input LoginInput) (Auth, error) {
	auth, err := s.backend.Login(ctx, meta, input)
	if err != nil {
		return Auth{}, err
	}
	s.setNativeLocale(auth.Identity.User.Locale)
	return auth, nil
}

// Logout 退出当前登录会话。
func (s *Service) Logout(ctx context.Context, meta RequestMeta) error {
	return s.backend.Logout(ctx, meta)
}

// LoadIdentity 返回当前登录身份。
func (s *Service) LoadIdentity(ctx context.Context, meta RequestMeta) (Identity, error) {
	identity, err := s.backend.LoadIdentity(ctx, meta)
	if err != nil {
		return Identity{}, err
	}
	s.setNativeLocale(identity.User.Locale)
	return identity, nil
}

// UpdateProfile 修改当前用户的头像、姓名和邮箱。
func (s *Service) UpdateProfile(ctx context.Context, meta RequestMeta, input ProfileInput) (CurrentUser, error) {
	return s.backend.UpdateProfile(ctx, meta, input)
}

// CreateFileUpload 创建文件上传请求。
func (s *Service) CreateFileUpload(ctx context.Context, meta RequestMeta, input FileUploadInput) (FileUpload, error) {
	return s.backend.CreateFileUpload(ctx, meta, input)
}

// CompleteFileUpload 核验并完成文件上传。
func (s *Service) CompleteFileUpload(ctx context.Context, meta RequestMeta, fileID string) (File, error) {
	return s.backend.CompleteFileUpload(ctx, meta, fileID)
}

// SelectProfileImage 在原生端选择并读取用户头像图片。
func (s *Service) SelectProfileImage(ctx context.Context, meta RequestMeta) (ProfileImageFile, error) {
	if s.profileImageSelector == nil {
		return ProfileImageFile{}, methodNotAllowedError(meta, "SelectProfileImage")
	}
	return s.profileImageSelector.SelectProfileImage(ctx, meta)
}

// ChangePassword 核验当前密码并保存新密码。
func (s *Service) ChangePassword(ctx context.Context, meta RequestMeta, input ChangePasswordInput) error {
	return s.backend.ChangePassword(ctx, meta, input)
}

// UpdateUserPreferences 保存当前用户的偏好设置。
func (s *Service) UpdateUserPreferences(ctx context.Context, meta RequestMeta, input UserPreferencesInput) (CurrentUser, error) {
	user, err := s.backend.UpdateUserPreferences(ctx, meta, input)
	if err != nil {
		return CurrentUser{}, err
	}
	s.setNativeLocale(user.Locale)
	return user, nil
}

// setNativeLocale 在当前平台支持时同步原生界面语言。
func (s *Service) setNativeLocale(locale Locale) {
	if s.nativeLocaleUpdater != nil {
		s.nativeLocaleUpdater.SetLocale(locale)
	}
}

// CheckNotificationPermission 返回当前设备的系统通知授权状态。
func (s *Service) CheckNotificationPermission(ctx context.Context, meta RequestMeta) (NotificationPermissionStatus, error) {
	if s.nativeNotification == nil {
		return NotificationPermissionStatusUnsupported, nil
	}
	return s.nativeNotification.CheckNotificationPermission(ctx, meta)
}

// RequestNotificationPermission 请求当前设备允许发送系统通知。
func (s *Service) RequestNotificationPermission(ctx context.Context, meta RequestMeta) (NotificationPermissionStatus, error) {
	if s.nativeNotification == nil {
		return NotificationPermissionStatusUnsupported, nil
	}
	return s.nativeNotification.RequestNotificationPermission(ctx, meta)
}

// SendMessageNotification 在当前设备投递一条新消息系统通知。
func (s *Service) SendMessageNotification(ctx context.Context, meta RequestMeta, input MessageNotificationInput) error {
	if s.nativeNotification == nil {
		return methodNotAllowedError(meta, "SendMessageNotification")
	}
	return s.nativeNotification.SendMessageNotification(ctx, meta, input)
}

// UpdateUnreadIndicator 更新当前设备的未读提示。
func (s *Service) UpdateUnreadIndicator(_ context.Context, meta RequestMeta, state UnreadIndicatorState) error {
	if s.unreadIndicator == nil {
		return methodNotAllowedError(meta, "UpdateUnreadIndicator")
	}
	return s.unreadIndicator.SetUnreadState(state)
}

// UpdateUserWorkStatus 保存当前用户主动设置的工作状态。
func (s *Service) UpdateUserWorkStatus(ctx context.Context, meta RequestMeta, input UserWorkStatusInput) (CurrentUser, error) {
	return s.backend.UpdateUserWorkStatus(ctx, meta, input)
}

// LoadInbox 返回当前用户的统一收件箱。
func (s *Service) LoadInbox(ctx context.Context, meta RequestMeta) (Inbox, error) {
	return s.backend.LoadInbox(ctx, meta)
}

// ListMessageChannels 返回消息渠道列表。
func (s *Service) ListMessageChannels(ctx context.Context, meta RequestMeta) (MessageChannelList, error) {
	return s.backend.ListMessageChannels(ctx, meta)
}

// GetWebsiteChannel 返回网站渠道详情。
func (s *Service) GetWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannel, error) {
	return s.backend.GetWebsiteChannel(ctx, meta, channelID)
}

// GetMessageChannel 返回消息渠道基础信息。
func (s *Service) GetMessageChannel(ctx context.Context, meta RequestMeta, channelID string) (MessageChannelSummary, error) {
	return s.backend.GetMessageChannel(ctx, meta, channelID)
}

// CreateMessageChannel 创建消息渠道。
func (s *Service) CreateMessageChannel(ctx context.Context, meta RequestMeta, input CreateMessageChannelInput) (MessageChannelSummary, error) {
	return s.backend.CreateMessageChannel(ctx, meta, input)
}

// UpdateMessageChannel 修改消息渠道基础信息。
func (s *Service) UpdateMessageChannel(ctx context.Context, meta RequestMeta, channelID string, input MessageChannelInput) (MessageChannelSummary, error) {
	return s.backend.UpdateMessageChannel(ctx, meta, channelID, input)
}

// UpdateWebsiteChannelChatInterface 修改网站渠道聊天界面。
func (s *Service) UpdateWebsiteChannelChatInterface(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelChatInterfaceInput) (WebsiteChannelChatInterface, error) {
	return s.backend.UpdateWebsiteChannelChatInterface(ctx, meta, channelID, input)
}

// UpdateWebsiteChannelAccess 修改网站渠道允许使用的网站。
func (s *Service) UpdateWebsiteChannelAccess(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelAccessInput) (WebsiteChannelAccess, error) {
	return s.backend.UpdateWebsiteChannelAccess(ctx, meta, channelID, input)
}

// DeactivateMessageChannel 停用消息渠道。
func (s *Service) DeactivateMessageChannel(ctx context.Context, meta RequestMeta, channelID string) (MessageChannelSummary, error) {
	return s.backend.DeactivateMessageChannel(ctx, meta, channelID)
}

// ActivateMessageChannel 启用消息渠道。
func (s *Service) ActivateMessageChannel(ctx context.Context, meta RequestMeta, channelID string) (MessageChannelSummary, error) {
	return s.backend.ActivateMessageChannel(ctx, meta, channelID)
}

// ListChannelOptions 返回当前企业的渠道选择项。
func (s *Service) ListChannelOptions(ctx context.Context, meta RequestMeta) (ChannelOptionList, error) {
	return s.backend.ListChannelOptions(ctx, meta)
}

// ListMemberOptions 返回可分配的企业成员和 AI 员工。
func (s *Service) ListMemberOptions(ctx context.Context, meta RequestMeta, input MemberOptionListInput) (MemberOptionList, error) {
	return s.backend.ListMemberOptions(ctx, meta, input)
}

// CreateAgent 创建企业 AI 员工。
func (s *Service) CreateAgent(ctx context.Context, meta RequestMeta, input CreateAgentInput) (Agent, error) {
	return s.backend.CreateAgent(ctx, meta, input)
}

// ListAgentModelOptions 返回 AI 员工可使用的对话模型。
func (s *Service) ListAgentModelOptions(ctx context.Context, meta RequestMeta) (AgentModelOptionList, error) {
	return s.backend.ListAgentModelOptions(ctx, meta)
}

// ListAgents 返回企业 AI 员工目录。
func (s *Service) ListAgents(ctx context.Context, meta RequestMeta, input AgentListInput) (AgentList, error) {
	return s.backend.ListAgents(ctx, meta, input)
}

// GetAgent 返回企业 AI 员工详情。
func (s *Service) GetAgent(ctx context.Context, meta RequestMeta, agentID string) (Agent, error) {
	return s.backend.GetAgent(ctx, meta, agentID)
}

// UpdateAgent 修改企业 AI 员工。
func (s *Service) UpdateAgent(ctx context.Context, meta RequestMeta, agentID string, input UpdateAgentInput) (Agent, error) {
	return s.backend.UpdateAgent(ctx, meta, agentID, input)
}

// UpdateAgentCapability 修改企业 AI 员工的能力配置。
func (s *Service) UpdateAgentCapability(ctx context.Context, meta RequestMeta, agentID string, input AgentCapabilityInput) (Agent, error) {
	return s.backend.UpdateAgentCapability(ctx, meta, agentID, input)
}

// UpdateAgentWorkStatus 修改企业 AI 员工工作状态。
func (s *Service) UpdateAgentWorkStatus(ctx context.Context, meta RequestMeta, agentID string, input AgentWorkStatusInput) (Agent, error) {
	return s.backend.UpdateAgentWorkStatus(ctx, meta, agentID, input)
}

// DeactivateAgent 禁用企业 AI 员工账号。
func (s *Service) DeactivateAgent(ctx context.Context, meta RequestMeta, agentID string) (Agent, error) {
	return s.backend.DeactivateAgent(ctx, meta, agentID)
}

// ReactivateAgent 恢复企业 AI 员工。
func (s *Service) ReactivateAgent(ctx context.Context, meta RequestMeta, agentID string) (Agent, error) {
	return s.backend.ReactivateAgent(ctx, meta, agentID)
}

// ListUsers 返回企业成员列表。
func (s *Service) ListUsers(ctx context.Context, meta RequestMeta, input UserListInput) (UserList, error) {
	return s.backend.ListUsers(ctx, meta, input)
}

// GetUser 返回企业成员详情。
func (s *Service) GetUser(ctx context.Context, meta RequestMeta, userID string) (User, error) {
	return s.backend.GetUser(ctx, meta, userID)
}

// CreateUser 创建企业成员账号。
func (s *Service) CreateUser(ctx context.Context, meta RequestMeta, input CreateUserInput) (User, error) {
	return s.backend.CreateUser(ctx, meta, input)
}

// UpdateUser 修改企业成员资料、角色和所属团队。
func (s *Service) UpdateUser(ctx context.Context, meta RequestMeta, userID string, input UpdateUserInput) (User, error) {
	return s.backend.UpdateUser(ctx, meta, userID, input)
}

// UpdateUserRoles 在一个事务中批量调整企业成员角色。
func (s *Service) UpdateUserRoles(ctx context.Context, meta RequestMeta, input UserRoleChangesInput) error {
	return s.backend.UpdateUserRoles(ctx, meta, input)
}

// DeactivateUser 禁用企业成员账号。
func (s *Service) DeactivateUser(ctx context.Context, meta RequestMeta, userID string) (User, error) {
	return s.backend.DeactivateUser(ctx, meta, userID)
}

// ReactivateUser 恢复企业成员账号。
func (s *Service) ReactivateUser(ctx context.Context, meta RequestMeta, userID string) (User, error) {
	return s.backend.ReactivateUser(ctx, meta, userID)
}

// ListTeams 返回企业团队列表。
func (s *Service) ListTeams(ctx context.Context, meta RequestMeta, input TeamListInput) (TeamList, error) {
	return s.backend.ListTeams(ctx, meta, input)
}

// CreateTeam 创建企业团队。
func (s *Service) CreateTeam(ctx context.Context, meta RequestMeta, input TeamInput) (Team, error) {
	return s.backend.CreateTeam(ctx, meta, input)
}

// UpdateTeam 修改企业团队。
func (s *Service) UpdateTeam(ctx context.Context, meta RequestMeta, teamID string, input TeamInput) (Team, error) {
	return s.backend.UpdateTeam(ctx, meta, teamID, input)
}

// DeleteTeam 删除企业团队及其成员关系。
func (s *Service) DeleteTeam(ctx context.Context, meta RequestMeta, teamID string) error {
	return s.backend.DeleteTeam(ctx, meta, teamID)
}

// ListKnowledgeBases 返回当前企业的知识库列表。
func (s *Service) ListKnowledgeBases(ctx context.Context, meta RequestMeta) (KnowledgeBaseList, error) {
	return s.backend.ListKnowledgeBases(ctx, meta)
}

// GetKnowledgeBase 返回当前企业中的知识库详情。
func (s *Service) GetKnowledgeBase(ctx context.Context, meta RequestMeta, knowledgeBaseID string) (KnowledgeBase, error) {
	return s.backend.GetKnowledgeBase(ctx, meta, knowledgeBaseID)
}

// CreateKnowledgeBase 创建企业知识库。
func (s *Service) CreateKnowledgeBase(ctx context.Context, meta RequestMeta, input KnowledgeBaseInput) (KnowledgeBase, error) {
	return s.backend.CreateKnowledgeBase(ctx, meta, input)
}

// UpdateKnowledgeBase 修改企业知识库。
func (s *Service) UpdateKnowledgeBase(ctx context.Context, meta RequestMeta, knowledgeBaseID string, input KnowledgeBaseInput) (KnowledgeBase, error) {
	return s.backend.UpdateKnowledgeBase(ctx, meta, knowledgeBaseID, input)
}

// DeleteKnowledgeBase 删除企业知识库。
func (s *Service) DeleteKnowledgeBase(ctx context.Context, meta RequestMeta, knowledgeBaseID string) error {
	return s.backend.DeleteKnowledgeBase(ctx, meta, knowledgeBaseID)
}

// CreateKnowledgeGroup 创建知识库分组。
func (s *Service) CreateKnowledgeGroup(ctx context.Context, meta RequestMeta, knowledgeBaseID string, input KnowledgeGroupInput) (KnowledgeBase, error) {
	return s.backend.CreateKnowledgeGroup(ctx, meta, knowledgeBaseID, input)
}

// UpdateKnowledgeGroup 修改知识库分组。
func (s *Service) UpdateKnowledgeGroup(ctx context.Context, meta RequestMeta, knowledgeBaseID, groupID string, input KnowledgeGroupInput) (KnowledgeBase, error) {
	return s.backend.UpdateKnowledgeGroup(ctx, meta, knowledgeBaseID, groupID, input)
}

// DeleteKnowledgeGroup 删除空知识库分组。
func (s *Service) DeleteKnowledgeGroup(ctx context.Context, meta RequestMeta, knowledgeBaseID, groupID string) (KnowledgeBase, error) {
	return s.backend.DeleteKnowledgeGroup(ctx, meta, knowledgeBaseID, groupID)
}

// ListTeamMembers 返回团队成员列表。
func (s *Service) ListTeamMembers(ctx context.Context, meta RequestMeta, teamID string, input TeamMemberListInput) (TeamMemberList, error) {
	return s.backend.ListTeamMembers(ctx, meta, teamID, input)
}

// ListTeamMemberCandidates 返回尚未加入团队的企业身份。
func (s *Service) ListTeamMemberCandidates(ctx context.Context, meta RequestMeta, teamID string, input TeamMemberCandidateInput) (TeamMemberCandidateList, error) {
	return s.backend.ListTeamMemberCandidates(ctx, meta, teamID, input)
}

// AddTeamMembers 将企业身份批量加入团队。
func (s *Service) AddTeamMembers(ctx context.Context, meta RequestMeta, teamID string, input TeamMemberInput) (Team, error) {
	return s.backend.AddTeamMembers(ctx, meta, teamID, input)
}

// RemoveTeamMembers 将企业身份批量移出团队。
func (s *Service) RemoveTeamMembers(ctx context.Context, meta RequestMeta, teamID string, input TeamMemberInput) (Team, error) {
	return s.backend.RemoveTeamMembers(ctx, meta, teamID, input)
}

// ListContacts 返回联系人列表。
func (s *Service) ListContacts(ctx context.Context, meta RequestMeta, input ContactListInput) (ContactList, error) {
	return s.backend.ListContacts(ctx, meta, input)
}

// GetContact 返回联系人详情。
func (s *Service) GetContact(ctx context.Context, meta RequestMeta, contactID string) (Contact, error) {
	return s.backend.GetContact(ctx, meta, contactID)
}

// CreateContact 创建联系人。
func (s *Service) CreateContact(ctx context.Context, meta RequestMeta, input ContactInput) (Contact, error) {
	return s.backend.CreateContact(ctx, meta, input)
}

// UpdateContact 修改联系人。
func (s *Service) UpdateContact(ctx context.Context, meta RequestMeta, contactID string, input ContactInput) (Contact, error) {
	return s.backend.UpdateContact(ctx, meta, contactID, input)
}

// DeleteContact 将联系人移入回收站。
func (s *Service) DeleteContact(ctx context.Context, meta RequestMeta, contactID string) error {
	return s.backend.DeleteContact(ctx, meta, contactID)
}

// RestoreContact 恢复联系人。
func (s *Service) RestoreContact(ctx context.Context, meta RequestMeta, contactID string) (Contact, error) {
	return s.backend.RestoreContact(ctx, meta, contactID)
}

// ListRoles 返回当前企业的角色和预定义权限目录。
func (s *Service) ListRoles(ctx context.Context, meta RequestMeta) (RoleList, error) {
	return s.backend.ListRoles(ctx, meta)
}

// GetRole 返回当前企业的角色详情。
func (s *Service) GetRole(ctx context.Context, meta RequestMeta, roleID string) (Role, error) {
	return s.backend.GetRole(ctx, meta, roleID)
}

// CreateRole 创建自定义角色。
func (s *Service) CreateRole(ctx context.Context, meta RequestMeta, input RoleInput) (Role, error) {
	return s.backend.CreateRole(ctx, meta, input)
}

// UpdateRole 修改角色信息和权限。
func (s *Service) UpdateRole(ctx context.Context, meta RequestMeta, roleID string, input RoleInput) (Role, error) {
	return s.backend.UpdateRole(ctx, meta, roleID, input)
}

// DeleteRole 删除自定义角色。
func (s *Service) DeleteRole(ctx context.Context, meta RequestMeta, roleID string) error {
	return s.backend.DeleteRole(ctx, meta, roleID)
}

// ListAIProviders 返回当前企业的模型服务供应商列表。
func (s *Service) ListAIProviders(ctx context.Context, meta RequestMeta) (AIProviderList, error) {
	return s.backend.ListAIProviders(ctx, meta)
}

// GetAIProvider 返回当前企业中的模型服务供应商详情。
func (s *Service) GetAIProvider(ctx context.Context, meta RequestMeta, providerID string) (AIProvider, error) {
	return s.backend.GetAIProvider(ctx, meta, providerID)
}

// ListAvailableAIModels 返回指定品牌的预设模型目录。
func (s *Service) ListAvailableAIModels(ctx context.Context, meta RequestMeta, brand AIProviderBrand) (AIProviderModelList, error) {
	return s.backend.ListAvailableAIModels(ctx, meta, brand)
}

// CreateAIProvider 创建模型服务供应商。
func (s *Service) CreateAIProvider(ctx context.Context, meta RequestMeta, input AIProviderInput) (AIProvider, error) {
	return s.backend.CreateAIProvider(ctx, meta, input)
}

// UpdateAIProvider 修改模型服务供应商。
func (s *Service) UpdateAIProvider(ctx context.Context, meta RequestMeta, providerID string, input AIProviderInput) (AIProvider, error) {
	return s.backend.UpdateAIProvider(ctx, meta, providerID, input)
}

// DeleteAIProvider 删除模型服务供应商。
func (s *Service) DeleteAIProvider(ctx context.Context, meta RequestMeta, providerID string) error {
	return s.backend.DeleteAIProvider(ctx, meta, providerID)
}

// UpdateOrganization 修改当前企业名称。
func (s *Service) UpdateOrganization(ctx context.Context, meta RequestMeta, input OrganizationInput) (Organization, error) {
	return s.backend.UpdateOrganization(ctx, meta, input)
}

// GetS3Setting 返回当前企业的对象存储设置。
func (s *Service) GetS3Setting(ctx context.Context, meta RequestMeta) (S3Setting, error) {
	return s.backend.GetS3Setting(ctx, meta)
}

// SaveS3Setting 保存当前企业的对象存储设置。
func (s *Service) SaveS3Setting(ctx context.Context, meta RequestMeta, input S3Setting) (S3Setting, error) {
	return s.backend.SaveS3Setting(ctx, meta, input)
}

// TestS3Setting 测试对象存储连接。
func (s *Service) TestS3Setting(ctx context.Context, meta RequestMeta, input S3Setting) error {
	return s.backend.TestS3Setting(ctx, meta, input)
}

// ServerURL 返回原生端当前配置的企业服务器地址。
func (s *Service) ServerURL(ctx context.Context, meta RequestMeta) (string, error) {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return "", methodNotAllowedError(meta, "ServerURL")
	}
	return connector.ServerURL(ctx, meta)
}

// ProbeServer 检测企业服务器并返回公开企业名称，不保存地址。
func (s *Service) ProbeServer(ctx context.Context, meta RequestMeta, serverURL string) (InstallationStatus, error) {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return InstallationStatus{}, methodNotAllowedError(meta, "ProbeServer")
	}
	return connector.ProbeServer(ctx, meta, serverURL)
}

// ConnectServer 验证并保存原生端企业服务器地址。
func (s *Service) ConnectServer(ctx context.Context, meta RequestMeta, serverURL string) error {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return methodNotAllowedError(meta, "ConnectServer")
	}
	return connector.ConnectServer(ctx, meta, serverURL)
}
