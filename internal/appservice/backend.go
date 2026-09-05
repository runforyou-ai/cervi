package appservice

import "context"

//go:generate go run github.com/runforyou-ai/cervi/internal/tools/appservicegen

// Backend 定义各运行平台都需要实现的业务调用。
//
// 每个方法必须携带一条 cervi:route 指令，格式为：
//
//	cervi:route <HTTP方法> <路径> [status=201] [query=<参数名>] [manual=service,api,proxy]
//
// appservicegen 按指令生成 Service 委托、Gin 路由和 API Proxy 转发；
// manual 标记的层由对应包手写实现。路径中的 :参数 依次对应签名中的 string 参数，
// GET 方法的结构体参数按 query 标签绑定查询参数，其余方法的结构体参数绑定 JSON 请求体。
type Backend interface {
	// InstallationStatus 返回服务端初始化状态和公开企业名称。
	//cervi:route GET /installation/status manual=proxy
	InstallationStatus(context.Context, RequestMeta) (InstallationStatus, error)
	// Login 校验账号密码并建立登录会话。
	//cervi:route POST /auth/login manual=service,proxy
	Login(context.Context, RequestMeta, LoginInput) (Auth, error)
	// Logout 退出当前登录会话。
	//cervi:route POST /auth/logout manual=proxy
	Logout(context.Context, RequestMeta) error
	// LoadIdentity 返回当前登录身份。
	//cervi:route GET /auth/identity manual=service
	LoadIdentity(context.Context, RequestMeta) (Identity, error)
	// UpdateProfile 修改当前用户的头像、姓名和邮箱。
	//cervi:route PATCH /profile
	UpdateProfile(context.Context, RequestMeta, ProfileInput) (CurrentUser, error)
	// CreateFileUpload 创建文件上传请求。
	//cervi:route POST /files/uploads status=201
	CreateFileUpload(context.Context, RequestMeta, FileUploadInput) (FileUpload, error)
	// CompleteFileUpload 核验并完成文件上传。
	//cervi:route POST /files/:fileID/complete
	CompleteFileUpload(context.Context, RequestMeta, string) (File, error)
	// ChangePassword 核验当前密码并保存新密码。
	//cervi:route PATCH /password
	ChangePassword(context.Context, RequestMeta, ChangePasswordInput) error
	// UpdateUserPreferences 保存当前用户的偏好设置。
	//cervi:route PATCH /preferences manual=service
	UpdateUserPreferences(context.Context, RequestMeta, UserPreferencesInput) (CurrentUser, error)
	// UpdateUserWorkStatus 保存当前用户主动设置的工作状态。
	//cervi:route PATCH /work-status
	UpdateUserWorkStatus(context.Context, RequestMeta, UserWorkStatusInput) (CurrentUser, error)
	// LoadInbox 返回当前用户的统一收件箱。
	//cervi:route GET /inbox
	LoadInbox(context.Context, RequestMeta, LoadInboxInput) (Inbox, error)
	// ListCustomerServiceAssignees 返回有效真人和 AI 客服。
	//cervi:route GET /inbox/assignees
	ListCustomerServiceAssignees(context.Context, RequestMeta) (CustomerServiceAssigneeList, error)
	// ListConversationMessages 返回成员可见的会话消息。
	//cervi:route GET /conversations/:conversationID/messages
	ListConversationMessages(context.Context, RequestMeta, string, ConversationMessageListInput) (ConversationMessageList, error)
	// GetConversationMessageContext 返回目标消息及其前后上下文。
	//cervi:route GET /conversations/:conversationID/messages/:messageID/context
	GetConversationMessageContext(context.Context, RequestMeta, string, string) (ConversationMessageList, error)
	// GetConversationNavigationState 返回群聊提及进度和最新可见消息。
	//cervi:route GET /conversations/:conversationID/navigation
	GetConversationNavigationState(context.Context, RequestMeta, string) (ConversationNavigationState, error)
	// ListPendingConversationMentions 返回本轮待查看提及目标。
	//cervi:route GET /conversations/:conversationID/mentions/pending
	ListPendingConversationMentions(context.Context, RequestMeta, string) (PendingConversationMentions, error)
	// MarkConversationMentionReviewed 连续确认群聊提及。
	//cervi:route POST /conversations/:conversationID/mentions/review
	MarkConversationMentionReviewed(context.Context, RequestMeta, string, MarkConversationMentionReviewedInput) (ConversationMentionReview, error)
	// MarkConversationRead 单调推进当前用户的原生会话已读水位。
	//cervi:route POST /conversations/:conversationID/read
	MarkConversationRead(context.Context, RequestMeta, string, MarkConversationReadInput) (ConversationReadState, error)
	// UpdateConversationNotificationSettings 保存当前用户的原生会话提醒设置。
	//cervi:route PATCH /conversations/:conversationID/notification-settings
	UpdateConversationNotificationSettings(context.Context, RequestMeta, string, ConversationNotificationSettingsInput) (ConversationNotificationSettings, error)
	// SendCustomerTextMessage 发送客户会话文本消息。
	//cervi:route POST /conversations/:conversationID/messages
	SendCustomerTextMessage(context.Context, RequestMeta, string, CustomerTextMessageInput) (ConversationMessage, error)
	// ClaimServiceSession 领取或接管客户会话最新处理周期。
	//cervi:route POST /conversations/:conversationID/claim
	ClaimServiceSession(context.Context, RequestMeta, string) (CustomerServiceSession, error)
	// TransferServiceSession 把当前负责的处理周期转给另一位客服。
	//cervi:route POST /conversations/:conversationID/transfer
	TransferServiceSession(context.Context, RequestMeta, string, TransferServiceSessionInput) (CustomerServiceSession, error)
	// CloseServiceSession 关闭客户会话最新处理周期。
	//cervi:route POST /conversations/:conversationID/close
	CloseServiceSession(context.Context, RequestMeta, string) (CustomerServiceSession, error)
	// ReopenServiceSession 重新打开客户会话最新处理周期并分配给当前身份。
	//cervi:route POST /conversations/:conversationID/reopen
	ReopenServiceSession(context.Context, RequestMeta, string) (CustomerServiceSession, error)
	// SendFirstDirectTextMessage 向目标身份发送首条单聊消息并按需创建长期会话。
	//cervi:route POST /direct-conversations/messages
	SendFirstDirectTextMessage(context.Context, RequestMeta, FirstDirectTextMessageInput) (FirstDirectTextMessageResult, error)
	// FindDirectConversation 按目标身份查找当前成员的活跃单聊。
	//cervi:route GET /direct-conversations/by-target/:targetIdentityID
	FindDirectConversation(context.Context, RequestMeta, string) (DirectConversationLookup, error)
	// SendDirectTextMessage 发送内部单聊文本消息。
	//cervi:route POST /direct-conversations/:conversationID/messages
	SendDirectTextMessage(context.Context, RequestMeta, string, DirectTextMessageInput) (ConversationMessage, error)
	// CreateGroupConversation 创建企业内部群聊。
	//cervi:route POST /group-conversations status=201
	CreateGroupConversation(context.Context, RequestMeta, GroupConversationInput) (InboxConversation, error)
	// GetGroupConversation 返回当前成员可见的群聊资料。
	//cervi:route GET /group-conversations/:conversationID
	GetGroupConversation(context.Context, RequestMeta, string) (GroupConversation, error)
	// UpdateGroupConversation 修改群聊资料。
	//cervi:route PATCH /group-conversations/:conversationID
	UpdateGroupConversation(context.Context, RequestMeta, string, GroupConversationProfileInput) (GroupConversation, error)
	// AddGroupConversationMembers 批量增加群聊成员。
	//cervi:route POST /group-conversations/:conversationID/members
	AddGroupConversationMembers(context.Context, RequestMeta, string, GroupConversationMembersInput) (GroupConversation, error)
	// RemoveGroupConversationMember 移除单个群聊成员。
	//cervi:route POST /group-conversations/:conversationID/members/remove
	RemoveGroupConversationMember(context.Context, RequestMeta, string, GroupConversationMemberInput) (GroupConversation, error)
	// TransferGroupConversationOwner 转让群主。
	//cervi:route POST /group-conversations/:conversationID/owner/transfer
	TransferGroupConversationOwner(context.Context, RequestMeta, string, GroupConversationOwnerInput) (GroupConversation, error)
	// LeaveGroupConversation 退出群聊并按需转让群主。
	//cervi:route POST /group-conversations/:conversationID/leave
	LeaveGroupConversation(context.Context, RequestMeta, string, GroupConversationLeaveInput) error
	// SendGroupTextMessage 发送企业内部群聊文本消息。
	//cervi:route POST /group-conversations/:conversationID/messages
	SendGroupTextMessage(context.Context, RequestMeta, string, GroupTextMessageInput) (ConversationMessage, error)
	// ListMessageChannels 返回消息渠道列表。
	//cervi:route GET /channels
	ListMessageChannels(context.Context, RequestMeta) (MessageChannelList, error)
	// GetWebsiteChannel 返回网站渠道详情。
	//cervi:route GET /channels/website/:channelID
	GetWebsiteChannel(context.Context, RequestMeta, string) (WebsiteChannel, error)
	// GetTelegramChannel 返回 Telegram 渠道详情。
	//cervi:route GET /channels/telegram/:channelID
	GetTelegramChannel(context.Context, RequestMeta, string) (TelegramChannel, error)
	// TestTelegramChannelConnection 测试 Telegram 草稿 Token。
	//cervi:route POST /channels/telegram/:channelID/connection/test
	TestTelegramChannelConnection(context.Context, RequestMeta, string, TelegramChannelConnectionTestInput) error
	// SaveTelegramChannelConnection 保存 Telegram 机器人和 Webhook 设置。
	//cervi:route PUT /channels/telegram/:channelID/connection
	SaveTelegramChannelConnection(context.Context, RequestMeta, string, TelegramChannelConnectionInput) (TelegramChannel, error)
	// GetMessageChannel 返回消息渠道基础信息。
	//cervi:route GET /channels/:channelID
	GetMessageChannel(context.Context, RequestMeta, string) (MessageChannelSummary, error)
	// CreateMessageChannel 创建消息渠道。
	//cervi:route POST /channels status=201
	CreateMessageChannel(context.Context, RequestMeta, CreateMessageChannelInput) (MessageChannelSummary, error)
	// UpdateMessageChannel 修改消息渠道基础信息。
	//cervi:route PUT /channels/:channelID
	UpdateMessageChannel(context.Context, RequestMeta, string, MessageChannelInput) (MessageChannelSummary, error)
	// UpdateWebsiteChannelChatInterface 修改网站渠道聊天界面。
	//cervi:route PUT /channels/website/:channelID/chat-interface
	UpdateWebsiteChannelChatInterface(context.Context, RequestMeta, string, WebsiteChannelChatInterfaceInput) (WebsiteChannelChatInterface, error)
	// UpdateWebsiteChannelAccess 修改网站渠道允许使用的网站。
	//cervi:route PUT /channels/website/:channelID/access
	UpdateWebsiteChannelAccess(context.Context, RequestMeta, string, WebsiteChannelAccessInput) (WebsiteChannelAccess, error)
	// DeactivateMessageChannel 停用消息渠道。
	//cervi:route POST /channels/:channelID/deactivate
	DeactivateMessageChannel(context.Context, RequestMeta, string) (MessageChannelSummary, error)
	// ActivateMessageChannel 启用消息渠道。
	//cervi:route POST /channels/:channelID/activate
	ActivateMessageChannel(context.Context, RequestMeta, string) (MessageChannelSummary, error)
	// ListChannelOptions 返回当前企业的渠道选择项。
	//cervi:route GET /channels/options
	ListChannelOptions(context.Context, RequestMeta) (ChannelOptionList, error)
	// ListMemberOptions 返回可分配的企业成员和 AI 员工。
	//cervi:route GET /members/options
	ListMemberOptions(context.Context, RequestMeta, MemberOptionListInput) (MemberOptionList, error)
	// ListAgentModelOptions 返回 AI 员工可使用的对话模型。
	//cervi:route GET /agents/model-options
	ListAgentModelOptions(context.Context, RequestMeta) (AgentModelOptionList, error)
	// CreateAgent 创建企业 AI 员工。
	//cervi:route POST /agents status=201
	CreateAgent(context.Context, RequestMeta, CreateAgentInput) (Agent, error)
	// ListAgents 返回企业 AI 员工目录。
	//cervi:route GET /agents
	ListAgents(context.Context, RequestMeta, AgentListInput) (AgentList, error)
	// GetAgent 返回企业 AI 员工详情。
	//cervi:route GET /agents/:agentID
	GetAgent(context.Context, RequestMeta, string) (Agent, error)
	// UpdateAgent 修改企业 AI 员工。
	//cervi:route PUT /agents/:agentID
	UpdateAgent(context.Context, RequestMeta, string, UpdateAgentInput) (Agent, error)
	// UpdateAgentExecution 修改企业 AI 员工的执行配置。
	//cervi:route PUT /agents/:agentID/execution
	UpdateAgentExecution(context.Context, RequestMeta, string, AgentExecutionInput) (Agent, error)
	// UpdateAgentWorkStatus 修改企业 AI 员工工作状态。
	//cervi:route PUT /agents/:agentID/work-status
	UpdateAgentWorkStatus(context.Context, RequestMeta, string, AgentWorkStatusInput) (Agent, error)
	// DeactivateAgent 禁用企业 AI 员工账号。
	//cervi:route POST /agents/:agentID/deactivate
	DeactivateAgent(context.Context, RequestMeta, string) (Agent, error)
	// ReactivateAgent 恢复企业 AI 员工。
	//cervi:route POST /agents/:agentID/reactivate
	ReactivateAgent(context.Context, RequestMeta, string) (Agent, error)
	// ListUsers 返回企业成员列表。
	//cervi:route GET /users
	ListUsers(context.Context, RequestMeta, UserListInput) (UserList, error)
	// GetUser 返回企业成员详情。
	//cervi:route GET /users/:userID
	GetUser(context.Context, RequestMeta, string) (User, error)
	// CreateUser 创建企业成员账号。
	//cervi:route POST /users status=201
	CreateUser(context.Context, RequestMeta, CreateUserInput) (User, error)
	// UpdateUser 修改企业成员资料、角色和所属团队。
	//cervi:route PUT /users/:userID
	UpdateUser(context.Context, RequestMeta, string, UpdateUserInput) (User, error)
	// UpdateRoleAssignments 在一个事务中批量调整真人和 AI 员工角色。
	//cervi:route PATCH /roles/assignments
	UpdateRoleAssignments(context.Context, RequestMeta, RoleAssignmentsInput) error
	// DeactivateUser 禁用企业成员账号。
	//cervi:route POST /users/:userID/deactivate
	DeactivateUser(context.Context, RequestMeta, string) (User, error)
	// ReactivateUser 恢复企业成员账号。
	//cervi:route POST /users/:userID/reactivate
	ReactivateUser(context.Context, RequestMeta, string) (User, error)
	// ListTeams 返回企业团队列表。
	//cervi:route GET /teams
	ListTeams(context.Context, RequestMeta, TeamListInput) (TeamList, error)
	// CreateTeam 创建企业团队。
	//cervi:route POST /teams status=201
	CreateTeam(context.Context, RequestMeta, TeamInput) (Team, error)
	// UpdateTeam 修改企业团队。
	//cervi:route PUT /teams/:teamID
	UpdateTeam(context.Context, RequestMeta, string, TeamInput) (Team, error)
	// DeleteTeam 删除企业团队及其成员关系。
	//cervi:route DELETE /teams/:teamID
	DeleteTeam(context.Context, RequestMeta, string) error
	// ListTeamMembers 返回团队成员列表。
	//cervi:route GET /teams/:teamID/members
	ListTeamMembers(context.Context, RequestMeta, string, TeamMemberListInput) (TeamMemberList, error)
	// ListTeamMemberCandidates 返回尚未加入团队的企业身份。
	//cervi:route GET /teams/:teamID/member-candidates
	ListTeamMemberCandidates(context.Context, RequestMeta, string, TeamMemberCandidateInput) (TeamMemberCandidateList, error)
	// AddTeamMembers 将企业身份批量加入团队。
	//cervi:route POST /teams/:teamID/members
	AddTeamMembers(context.Context, RequestMeta, string, TeamMemberInput) (Team, error)
	// RemoveTeamMembers 将企业身份批量移出团队。
	//cervi:route POST /teams/:teamID/members/remove
	RemoveTeamMembers(context.Context, RequestMeta, string, TeamMemberInput) (Team, error)
	// ListKnowledgeBases 返回当前企业的知识库列表。
	//cervi:route GET /knowledge-bases
	ListKnowledgeBases(context.Context, RequestMeta) (KnowledgeBaseList, error)
	// ListExternalKnowledgeBaseOptions 返回指定连接可访问的外部知识库选项。
	//cervi:route GET /integration-connections/:connectionID/knowledge-bases
	ListExternalKnowledgeBaseOptions(context.Context, RequestMeta, string) (ExternalKnowledgeBaseOptionList, error)
	// ListKnowledgeDocuments 返回指定外部知识库的文档列表。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/documents
	ListKnowledgeDocuments(context.Context, RequestMeta, string, KnowledgeDocumentListInput) (KnowledgeDocumentList, error)
	// GetKnowledgeDocument 返回指定外部知识文档详情。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/documents/:documentID
	GetKnowledgeDocument(context.Context, RequestMeta, string, string) (KnowledgeDocument, error)
	// ListKnowledgeDocumentSegments 返回指定外部知识文档的分段列表。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/documents/:documentID/segments
	ListKnowledgeDocumentSegments(context.Context, RequestMeta, string, string, KnowledgeDocumentSegmentListInput) (KnowledgeDocumentSegmentList, error)
	// RetrieveKnowledgeBase 检索指定外部知识库。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/retrieve
	RetrieveKnowledgeBase(context.Context, RequestMeta, string, KnowledgeRetrievalInput) (KnowledgeRetrievalResult, error)
	// GetKnowledgeBase 返回当前企业中的知识库详情。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID
	GetKnowledgeBase(context.Context, RequestMeta, string) (KnowledgeBase, error)
	// CreateKnowledgeBase 创建企业知识库。
	//cervi:route POST /knowledge-bases status=201
	CreateKnowledgeBase(context.Context, RequestMeta, KnowledgeBaseInput) (KnowledgeBase, error)
	// UpdateKnowledgeBase 修改企业知识库。
	//cervi:route PUT /knowledge-bases/:knowledgeBaseID
	UpdateKnowledgeBase(context.Context, RequestMeta, string, KnowledgeBaseInput) (KnowledgeBase, error)
	// DeleteKnowledgeBase 删除企业知识库。
	//cervi:route DELETE /knowledge-bases/:knowledgeBaseID
	DeleteKnowledgeBase(context.Context, RequestMeta, string) error
	// CreateKnowledgeGroup 创建知识库分组。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/groups status=201
	CreateKnowledgeGroup(context.Context, RequestMeta, string, KnowledgeGroupInput) (KnowledgeBase, error)
	// UpdateKnowledgeGroup 修改知识库分组。
	//cervi:route PUT /knowledge-bases/:knowledgeBaseID/groups/:groupID
	UpdateKnowledgeGroup(context.Context, RequestMeta, string, string, KnowledgeGroupInput) (KnowledgeBase, error)
	// DeleteKnowledgeGroup 删除不含子分组的知识库分组。
	//cervi:route DELETE /knowledge-bases/:knowledgeBaseID/groups/:groupID
	DeleteKnowledgeGroup(context.Context, RequestMeta, string, string) (KnowledgeBase, error)
	// ListContacts 返回联系人列表。
	//cervi:route GET /contacts manual=api,proxy
	ListContacts(context.Context, RequestMeta, ContactListInput) (ContactList, error)
	// GetContact 返回联系人详情。
	//cervi:route GET /contacts/:contactID
	GetContact(context.Context, RequestMeta, string) (Contact, error)
	// CreateContact 创建联系人。
	//cervi:route POST /contacts status=201
	CreateContact(context.Context, RequestMeta, ContactInput) (Contact, error)
	// UpdateContact 修改联系人。
	//cervi:route PUT /contacts/:contactID
	UpdateContact(context.Context, RequestMeta, string, ContactInput) (Contact, error)
	// DeleteContact 将联系人移入回收站。
	//cervi:route DELETE /contacts/:contactID
	DeleteContact(context.Context, RequestMeta, string) error
	// RestoreContact 恢复联系人。
	//cervi:route POST /contacts/:contactID/restore
	RestoreContact(context.Context, RequestMeta, string) (Contact, error)
	// ListRoles 返回当前企业的角色和预定义权限目录。
	//cervi:route GET /settings/roles
	ListRoles(context.Context, RequestMeta) (RoleList, error)
	// GetRole 返回当前企业的角色详情。
	//cervi:route GET /settings/roles/:roleID
	GetRole(context.Context, RequestMeta, string) (Role, error)
	// CreateRole 创建自定义角色。
	//cervi:route POST /settings/roles status=201
	CreateRole(context.Context, RequestMeta, RoleInput) (Role, error)
	// UpdateRole 修改角色信息和权限。
	//cervi:route PUT /settings/roles/:roleID
	UpdateRole(context.Context, RequestMeta, string, RoleInput) (Role, error)
	// DeleteRole 删除自定义角色。
	//cervi:route DELETE /settings/roles/:roleID
	DeleteRole(context.Context, RequestMeta, string) error
	// ListAIProviders 返回当前企业的模型服务供应商列表。
	//cervi:route GET /integrations/model-services
	ListAIProviders(context.Context, RequestMeta) (AIProviderList, error)
	// GetAIProvider 返回当前企业中的模型服务供应商详情。
	//cervi:route GET /integrations/model-services/:providerID
	GetAIProvider(context.Context, RequestMeta, string) (AIProvider, error)
	// ListAvailableAIModels 返回指定品牌的预设模型目录。
	//cervi:route GET /integrations/model-services/models query=brand
	ListAvailableAIModels(context.Context, RequestMeta, AIProviderBrand) (AIProviderModelList, error)
	// TestAIProviderConnection 测试模型服务供应商草稿配置。
	//cervi:route POST /integrations/model-services/test
	TestAIProviderConnection(context.Context, RequestMeta, AIProviderConnectionInput) error
	// CreateAIProvider 创建模型服务供应商。
	//cervi:route POST /integrations/model-services status=201
	CreateAIProvider(context.Context, RequestMeta, AIProviderInput) (AIProvider, error)
	// UpdateAIProvider 修改模型服务供应商。
	//cervi:route PUT /integrations/model-services/:providerID
	UpdateAIProvider(context.Context, RequestMeta, string, AIProviderInput) (AIProvider, error)
	// DeleteAIProvider 删除模型服务供应商。
	//cervi:route DELETE /integrations/model-services/:providerID
	DeleteAIProvider(context.Context, RequestMeta, string) error
	// ListBusinessSystems 返回当前企业配置的业务系统。
	//cervi:route GET /integrations/business-systems
	ListBusinessSystems(context.Context, RequestMeta) (BusinessSystemList, error)
	// GetBusinessSystem 返回当前企业中的业务系统详情。
	//cervi:route GET /integrations/business-systems/:businessSystemID
	GetBusinessSystem(context.Context, RequestMeta, string) (BusinessSystem, error)
	// CreateBusinessSystem 创建业务系统。
	//cervi:route POST /integrations/business-systems status=201
	CreateBusinessSystem(context.Context, RequestMeta, BusinessSystemInput) (BusinessSystem, error)
	// UpdateBusinessSystem 修改业务系统。
	//cervi:route PUT /integrations/business-systems/:businessSystemID
	UpdateBusinessSystem(context.Context, RequestMeta, string, BusinessSystemInput) (BusinessSystem, error)
	// DeleteBusinessSystem 删除业务系统。
	//cervi:route DELETE /integrations/business-systems/:businessSystemID
	DeleteBusinessSystem(context.Context, RequestMeta, string) error
	// ListIntegrationConnections 返回当前企业的连接器列表。
	//cervi:route GET /integrations/connectors
	ListIntegrationConnections(context.Context, RequestMeta) (IntegrationConnectionList, error)
	// GetIntegrationConnection 返回当前企业中的连接器详情。
	//cervi:route GET /integrations/connectors/:connectionID
	GetIntegrationConnection(context.Context, RequestMeta, string) (IntegrationConnection, error)
	// TestIntegrationConnection 测试连接器草稿配置。
	//cervi:route POST /integrations/connectors/test
	TestIntegrationConnection(context.Context, RequestMeta, IntegrationConnectionTestInput) error
	// CreateIntegrationConnection 创建外部系统连接器。
	//cervi:route POST /integrations/connectors status=201
	CreateIntegrationConnection(context.Context, RequestMeta, IntegrationConnectionInput) (IntegrationConnection, error)
	// UpdateIntegrationConnection 修改外部系统连接器。
	//cervi:route PUT /integrations/connectors/:connectionID
	UpdateIntegrationConnection(context.Context, RequestMeta, string, IntegrationConnectionInput) (IntegrationConnection, error)
	// DeleteIntegrationConnection 删除外部系统连接器。
	//cervi:route DELETE /integrations/connectors/:connectionID
	DeleteIntegrationConnection(context.Context, RequestMeta, string) error
	// UpdateOrganization 修改当前企业通用设置。
	//cervi:route PUT /settings/organization
	UpdateOrganization(context.Context, RequestMeta, OrganizationInput) (Organization, error)
	// GetS3Setting 返回当前企业的对象存储设置。
	//cervi:route GET /settings/storage/s3
	GetS3Setting(context.Context, RequestMeta) (S3Setting, error)
	// SaveS3Setting 保存当前企业的对象存储设置。
	//cervi:route PUT /settings/storage/s3
	SaveS3Setting(context.Context, RequestMeta, S3SettingInput) (S3Setting, error)
	// TestS3Setting 测试对象存储连接。
	//cervi:route POST /settings/storage/s3/test
	TestS3Setting(context.Context, RequestMeta, S3SettingInput) error
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

// ImageSelector 由支持原生文件对话框的平台实现。
type ImageSelector interface {
	SelectImage(context.Context, RequestMeta) (ImageFile, error)
}

// ExternalPageOpener 由支持多窗口的平台实现，在应用内新窗口打开外部页面。
type ExternalPageOpener interface {
	OpenExternalPage(context.Context, RequestMeta, ExternalPageInput) error
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
