package appservice

import "context"

//go:generate go run github.com/runforyou-ai/cervi/internal/tools/appservicegen

// Backend 定义各运行平台都需要实现的业务调用。
//
// 每个方法必须携带一条 cervi:route 指令，格式为：
//
//	cervi:route <HTTP方法> <路径> [status=201] [query=<参数名>] [auth=public] [manual=service,api,proxy]
//
// appservicegen 按指令生成 Service 委托、Gin 路由、API Proxy 转发和服务端认证分发；
// manual 标记的层由对应包手写实现。路径中的 :参数 依次对应签名中的 string 参数，
// GET 方法的结构体参数按 query 标签绑定查询参数，其余方法的结构体参数绑定 JSON 请求体。
//
// auth 默认为 member：服务端分发层先解析登录身份，再把身份交给业务实现，
// 业务实现不重复处理认证。无需登录身份的方法标记 auth=public。
type Backend interface {
	// InstallationStatus 返回服务端初始化状态和公开企业名称。
	//cervi:route GET /installation/status auth=public manual=proxy
	InstallationStatus(context.Context, RequestMeta) (InstallationStatus, error)
	// Login 校验账号密码并建立登录会话。
	//cervi:route POST /auth/login auth=public manual=service,proxy
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
	// CreateFilePartUpload 创建一个分片的直传请求。
	//cervi:route POST /files/:fileID/parts
	CreateFilePartUpload(context.Context, RequestMeta, string, FilePartUploadInput) (FileUploadRequest, error)
	// PrepareFileUpload 为已有文件记录准备直传请求。
	//cervi:route POST /files/:fileID/upload
	PrepareFileUpload(context.Context, RequestMeta, string) (FileUpload, error)
	// CancelFileUpload 将未发送的临时文件交给清理任务。
	//cervi:route DELETE /files/:fileID/upload
	CancelFileUpload(context.Context, RequestMeta, string) error
	// SendAttachmentMessage 发送已上传的单聊、群聊或 AI 聊天附件消息，首发时创建会话。
	//cervi:route POST /conversation-attachments status=201
	SendAttachmentMessage(context.Context, RequestMeta, AttachmentMessageInput) (AttachmentMessageResult, error)
	// GetAttachmentDownload 签发当前成员可见消息附件的下载地址。
	//cervi:route GET /conversations/:conversationID/messages/:messageID/attachment
	GetAttachmentDownload(context.Context, RequestMeta, string, string) (FileDownload, error)
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
	// GetInboxContext 返回会话锚点的当前资格和原位置邻域。
	//cervi:route POST /inbox/context/query
	GetInboxContext(context.Context, RequestMeta, InboxContextInput) (InboxContext, error)
	// ReadInboxWindow 重读已加载双向边界之间的完整列表范围。
	//cervi:route POST /inbox/window/query
	ReadInboxWindow(context.Context, RequestMeta, InboxWindowInput) (InboxWindow, error)
	// GetInboxConversation 返回当前用户有权阅读的独立会话摘要。
	//cervi:route GET /conversations/:conversationID/summary
	GetInboxConversation(context.Context, RequestMeta, string) (InboxConversation, error)
	// ReadInboxConversations 按 ID 批量返回会话摘要及当前筛选资格。
	//cervi:route POST /inbox/conversations/query
	ReadInboxConversations(context.Context, RequestMeta, ReadInboxConversationsInput) (InboxConversationResults, error)
	// SearchInbox 按范围检索会话名称、消息正文与附件文件名、成员和外部联系人。
	//cervi:route GET /inbox/search
	SearchInbox(context.Context, RequestMeta, InboxSearchInput) (InboxSearchResult, error)
	// ListCustomerServiceAssignees 返回有效真人和 AI 客服。
	//cervi:route GET /inbox/assignees
	ListCustomerServiceAssignees(context.Context, RequestMeta) (CustomerServiceAssigneeList, error)
	// ListServiceQueueTeams 返回可作为客服队列的团队，本人所在团队排在前面。
	//cervi:route GET /inbox/queue-teams
	ListServiceQueueTeams(context.Context, RequestMeta) (ServiceQueueTeamList, error)
	// ListInboxChannels 返回收件箱渠道筛选候选，含已停用渠道。
	//cervi:route GET /inbox/channels
	ListInboxChannels(context.Context, RequestMeta) (InboxChannelList, error)
	// GetSyncHeads 返回当前用户可见会话与身份资料的同步探针值。
	//cervi:route GET /sync/heads
	GetSyncHeads(context.Context, RequestMeta) (SyncHeads, error)
	// ListConversationMessages 返回成员可见的会话消息。
	//cervi:route GET /conversations/:conversationID/messages
	ListConversationMessages(context.Context, RequestMeta, string, ConversationMessageListInput) (ConversationMessageList, error)
	// ReadConversationMessageWindow 重读已加载首尾游标之间的完整消息范围。
	//cervi:route GET /conversations/:conversationID/message-window
	ReadConversationMessageWindow(context.Context, RequestMeta, string, ConversationMessageWindowInput) (ConversationMessageList, error)
	// ListConversationMessageReferences 读取当前窗口的引用摘要和回复可用状态。
	//cervi:route GET /conversations/:conversationID/message-references
	ListConversationMessageReferences(context.Context, RequestMeta, string, ConversationMessageReferenceListInput) (ConversationMessageReferenceList, error)
	// GetConversationMessageContext 返回目标消息及其前后上下文。
	//cervi:route GET /conversations/:conversationID/messages/:messageID/context
	GetConversationMessageContext(context.Context, RequestMeta, string, string) (ConversationMessageList, error)
	// GetConversationNavigationState 返回群聊提及进度和最新可见消息。
	//cervi:route GET /conversations/:conversationID/navigation
	GetConversationNavigationState(context.Context, RequestMeta, string) (ConversationNavigationState, error)
	// ListPendingConversationMentions 返回本轮待查看提及目标。
	//cervi:route GET /conversations/:conversationID/mentions/pending
	ListPendingConversationMentions(context.Context, RequestMeta, string) (PendingConversationMentions, error)
	// MarkConversationMentionReviewed 确认已查看的群聊提及。
	//cervi:route POST /conversations/:conversationID/mentions/review
	MarkConversationMentionReviewed(context.Context, RequestMeta, string, MarkConversationMentionReviewedInput) (ConversationMentionReview, error)
	// MarkConversationRead 单调推进当前用户的会话已读水位。
	//cervi:route POST /conversations/:conversationID/read
	MarkConversationRead(context.Context, RequestMeta, string, MarkConversationReadInput) (ConversationReadState, error)
	// ReportConversationTyping 发布当前用户的输入状态：单聊与群聊发给其他真人成员，网站渠道客户会话发给该线程访客。
	//cervi:route POST /conversations/:conversationID/typing
	ReportConversationTyping(context.Context, RequestMeta, string, ConversationTypingInput) error
	// UpdateConversationUnreadMark 保存当前用户独立于阅读水位的未读标记。
	//cervi:route PATCH /conversations/:conversationID/unread-mark
	UpdateConversationUnreadMark(context.Context, RequestMeta, string, ConversationUnreadMarkInput) error
	// UpdateConversationPin 保存当前用户的会话置顶事实与置顶顺序。
	//cervi:route PATCH /conversations/:conversationID/pin
	UpdateConversationPin(context.Context, RequestMeta, string, ConversationPinInput) (ConversationPinState, error)
	// UpdateConversationNotificationSettings 保存当前用户的原生会话提醒设置。
	//cervi:route PATCH /conversations/:conversationID/notification-settings
	UpdateConversationNotificationSettings(context.Context, RequestMeta, string, ConversationNotificationSettingsInput) (ConversationNotificationSettings, error)
	// SendCustomerTextMessage 发送客户会话文本消息。
	//cervi:route POST /conversations/:conversationID/messages
	SendCustomerTextMessage(context.Context, RequestMeta, string, CustomerTextMessageInput) (ConversationMessage, error)
	// SendCustomerAttachmentMessage 发送客户会话附件消息。
	//cervi:route POST /conversations/:conversationID/attachment-messages
	SendCustomerAttachmentMessage(context.Context, RequestMeta, string, CustomerAttachmentMessageInput) (ConversationMessage, error)
	// ListCustomerReplyAgents 返回可用于 AI 写回复的 AI 员工。
	//cervi:route GET /reply-suggestion-agents
	ListCustomerReplyAgents(context.Context, RequestMeta) (CustomerReplyAgentList, error)
	// GenerateCustomerReplySuggestions 使用 AI 员工为客户会话生成对客回复候选。
	//cervi:route POST /conversations/:conversationID/reply-suggestions
	GenerateCustomerReplySuggestions(context.Context, RequestMeta, string, CustomerReplySuggestionsInput) (CustomerReplySuggestions, error)
	// ListCustomerCopilotThreads 返回客户会话的全部 Copilot 线程。
	//cervi:route GET /conversations/:conversationID/copilot-threads
	ListCustomerCopilotThreads(context.Context, RequestMeta, string) (CustomerCopilotThreadList, error)
	// SendFirstCustomerCopilotMessage 以首条提问创建客户会话的 Copilot 线程。
	//cervi:route POST /conversations/:conversationID/copilot-threads
	SendFirstCustomerCopilotMessage(context.Context, RequestMeta, string, FirstCustomerCopilotMessageInput) (FirstCustomerCopilotMessageResult, error)
	// SendCustomerCopilotTextMessage 向 Copilot 线程发送提问。
	//cervi:route POST /copilot-threads/:threadID/messages
	SendCustomerCopilotTextMessage(context.Context, RequestMeta, string, CustomerCopilotTextMessageInput) (ConversationMessage, error)
	// StopCustomerCopilotReply 停止 Copilot 线程中指定的回复并返回实际运行状态。
	//cervi:route POST /copilot-threads/:threadID/runs/:runID/stop
	StopCustomerCopilotReply(context.Context, RequestMeta, string, string) (AgentRunStatus, error)
	// ListCustomerMessageDeliveries 读取当前窗口的外部投递状态。
	//cervi:route GET /conversations/:conversationID/deliveries
	ListCustomerMessageDeliveries(context.Context, RequestMeta, string, CustomerDeliveryListInput) (CustomerDeliveryList, error)
	// ResolveCustomerMessageDelivery 人工处理失败或待确认的投递。
	//cervi:route POST /conversations/:conversationID/deliveries/:deliveryID/resolve
	ResolveCustomerMessageDelivery(context.Context, RequestMeta, string, string, CustomerDeliveryResolveInput) error
	// ClaimServiceSession 领取或接管客户会话最新处理周期。
	//cervi:route POST /conversations/:conversationID/claim
	ClaimServiceSession(context.Context, RequestMeta, string) (CustomerServiceSession, error)
	// TransferServiceSession 把当前负责的处理周期转给成员、团队队列或公共队列。
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
	// SendFirstAgentTextMessage 在首次发送时创建独立 AI 聊天。
	//cervi:route POST /agent-conversations/messages
	SendFirstAgentTextMessage(context.Context, RequestMeta, FirstAgentTextMessageInput) (FirstAgentTextMessageResult, error)
	// SendAgentTextMessage 向已有 AI 会话发送文本消息。
	//cervi:route POST /agent-conversations/:conversationID/messages
	SendAgentTextMessage(context.Context, RequestMeta, string, AgentTextMessageInput) (ConversationMessage, error)
	// StopAgentReply 停止独立 AI 会话中指定的回复并返回实际运行状态。
	//cervi:route POST /agent-conversations/:conversationID/runs/:runID/stop
	StopAgentReply(context.Context, RequestMeta, string, string) (AgentRunStatus, error)
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
	// LeaveGroupConversation 退出普通成员参与的群聊。
	//cervi:route POST /group-conversations/:conversationID/leave
	LeaveGroupConversation(context.Context, RequestMeta, string) error
	// DissolveGroupConversation 解散群聊并保留当前成员的只读历史。
	//cervi:route POST /group-conversations/:conversationID/dissolve
	DissolveGroupConversation(context.Context, RequestMeta, string) (GroupConversation, error)
	// SendGroupTextMessage 发送企业内部群聊文本消息。
	//cervi:route POST /group-conversations/:conversationID/messages
	SendGroupTextMessage(context.Context, RequestMeta, string, GroupTextMessageInput) (ConversationMessage, error)
	// StopGroupAgentReply 停止群聊中指定的 AI 员工回复并返回实际运行状态。
	//cervi:route POST /group-conversations/:conversationID/runs/:runID/stop
	StopGroupAgentReply(context.Context, RequestMeta, string, string) (AgentRunStatus, error)
	// GetAgentRunProcess 返回一次已完成运行的有序过程内容和模型用量。
	//cervi:route GET /agent-runs/:runID/process
	GetAgentRunProcess(context.Context, RequestMeta, string) (AgentRunProcess, error)
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
	// ListAgentMCPServerOptions 返回当前企业可配置的 MCP 服务。
	//cervi:route GET /agents/mcp-server-options
	ListAgentMCPServerOptions(context.Context, RequestMeta) (AgentMCPServerOptionList, error)
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
	UpdateAgentExecution(context.Context, RequestMeta, string, UpdateAgentExecutionInput) (Agent, error)
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
	// UpdateRoleAssignments 在一个事务中批量调整成员角色。
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
	// GetTeam 返回团队详情。
	//cervi:route GET /teams/:teamID
	GetTeam(context.Context, RequestMeta, string) (Team, error)
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
	// RetryKnowledgeDocument 按当前配置重新处理文档。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/documents/:documentID/retry
	RetryKnowledgeDocument(context.Context, RequestMeta, string, string) error
	// ListKnowledgeDocumentSegments 返回固定批次的分段页或锚点所在页。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/documents/:documentID/segments
	ListKnowledgeDocumentSegments(context.Context, RequestMeta, string, string, KnowledgeDocumentSegmentInput) (KnowledgeDocumentSegmentPage, error)
	// RetrieveKnowledgeBase 在指定知识库中执行检索测试，返回混合召回与重排后的分段。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/retrieval
	RetrieveKnowledgeBase(context.Context, RequestMeta, string, KnowledgeRetrievalInput) (KnowledgeRetrievalResult, error)
	// ListKnowledgeDocuments 返回当前分组的文档列表。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/documents
	ListKnowledgeDocuments(context.Context, RequestMeta, string, KnowledgeDocumentListInput) (KnowledgeDocumentList, error)
	// GetKnowledgeDocument 返回文档详情。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/documents/:documentID
	GetKnowledgeDocument(context.Context, RequestMeta, string, string) (KnowledgeDocument, error)
	// CreateKnowledgeDocuments 保存最多十个已上传的文档原件。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/documents status=201
	CreateKnowledgeDocuments(context.Context, RequestMeta, string, KnowledgeDocumentBatchInput) (KnowledgeDocumentBatch, error)
	// DeleteKnowledgeDocument 删除文档并释放原件。
	//cervi:route DELETE /knowledge-bases/:knowledgeBaseID/documents/:documentID
	DeleteKnowledgeDocument(context.Context, RequestMeta, string, string) error
	// GetKnowledgeDocumentPreview 签发当前文档的原件预览请求。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/documents/:documentID/preview
	GetKnowledgeDocumentPreview(context.Context, RequestMeta, string, string) (KnowledgeDocumentPreviewRequest, error)
	// CreateKnowledgeTextDocument 创建在线编写的文档并安排索引。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/text-documents status=201
	CreateKnowledgeTextDocument(context.Context, RequestMeta, string, KnowledgeTextDocumentInput) (KnowledgeDocument, error)
	// GetKnowledgeDocumentContent 返回在线文档正文或网页抓取快照。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/documents/:documentID/content
	GetKnowledgeDocumentContent(context.Context, RequestMeta, string, string) (KnowledgeDocumentContent, error)
	// UpdateKnowledgeDocumentContent 修改在线文档的名称与正文并安排索引。
	//cervi:route PUT /knowledge-bases/:knowledgeBaseID/documents/:documentID/content
	UpdateKnowledgeDocumentContent(context.Context, RequestMeta, string, string, KnowledgeDocumentContentInput) (KnowledgeDocument, error)
	// RenameKnowledgeDocument 修改在线文档或网页文档的名称。
	//cervi:route PUT /knowledge-bases/:knowledgeBaseID/documents/:documentID
	RenameKnowledgeDocument(context.Context, RequestMeta, string, string, KnowledgeDocumentRenameInput) (KnowledgeDocument, error)
	// CreateKnowledgeWebDocument 导入网页并安排首次抓取。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/web-documents status=201
	CreateKnowledgeWebDocument(context.Context, RequestMeta, string, KnowledgeWebDocumentInput) (KnowledgeDocument, error)
	// RefetchKnowledgeDocument 重新抓取网页文档并重新索引。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/documents/:documentID/refetch
	RefetchKnowledgeDocument(context.Context, RequestMeta, string, string, KnowledgeDocumentRefetchInput) error

	// ListKnowledgeQAEntries 返回分组中的本地问答列表。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/qa-entries
	ListKnowledgeQAEntries(context.Context, RequestMeta, string, KnowledgeQAListInput) (KnowledgeQAList, error)
	// GetKnowledgeQAEntry 返回完整的本地问答。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/qa-entries/:entryID
	GetKnowledgeQAEntry(context.Context, RequestMeta, string, string) (KnowledgeQAEntry, error)
	// CreateKnowledgeQAEntry 创建本地问答。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/qa-entries status=201
	CreateKnowledgeQAEntry(context.Context, RequestMeta, string, KnowledgeQAInput) (KnowledgeQAEntry, error)
	// UpdateKnowledgeQAEntry 修改本地问答。
	//cervi:route PUT /knowledge-bases/:knowledgeBaseID/qa-entries/:entryID
	UpdateKnowledgeQAEntry(context.Context, RequestMeta, string, string, KnowledgeQAInput) (KnowledgeQAEntry, error)
	// DeleteKnowledgeQAEntry 删除本地问答。
	//cervi:route DELETE /knowledge-bases/:knowledgeBaseID/qa-entries/:entryID
	DeleteKnowledgeQAEntry(context.Context, RequestMeta, string, string) error
	// RetryKnowledgeQAEntry 按当前配置重新索引问答。
	//cervi:route POST /knowledge-bases/:knowledgeBaseID/qa-entries/:entryID/retry
	RetryKnowledgeQAEntry(context.Context, RequestMeta, string, string) error
	// ListKnowledgeBases 返回当前企业的知识库列表。
	//cervi:route GET /knowledge-bases
	ListKnowledgeBases(context.Context, RequestMeta) (KnowledgeBaseList, error)
	// GetKnowledgeBase 返回当前企业中的知识库详情。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID
	GetKnowledgeBase(context.Context, RequestMeta, string) (KnowledgeBase, error)
	// ListKnowledgeBaseAgents 返回当前配置版本绑定知识库的 AI 员工。
	//cervi:route GET /knowledge-bases/:knowledgeBaseID/agents
	ListKnowledgeBaseAgents(context.Context, RequestMeta, string) (KnowledgeBaseAgentList, error)
	// CreateKnowledgeBase 创建企业知识库。
	//cervi:route POST /knowledge-bases status=201
	CreateKnowledgeBase(context.Context, RequestMeta, KnowledgeBaseInput) (KnowledgeBase, error)
	// UpdateKnowledgeBase 修改企业知识库。
	//cervi:route PUT /knowledge-bases/:knowledgeBaseID
	UpdateKnowledgeBase(context.Context, RequestMeta, string, KnowledgeBaseInput) (KnowledgeBase, error)
	// DeleteKnowledgeBase 删除企业知识库。
	//cervi:route DELETE /knowledge-bases/:knowledgeBaseID
	DeleteKnowledgeBase(context.Context, RequestMeta, string) error
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
	//cervi:route GET /settings/model-services
	ListAIProviders(context.Context, RequestMeta) (AIProviderList, error)
	// GetAIProvider 返回当前企业中的模型服务供应商详情。
	//cervi:route GET /settings/model-services/:providerID
	GetAIProvider(context.Context, RequestMeta, string) (AIProvider, error)
	// ListAvailableAIModels 返回指定品牌的预设模型目录。
	//cervi:route GET /settings/model-services/models query=brand
	ListAvailableAIModels(context.Context, RequestMeta, AIProviderBrand) (AIProviderModelList, error)
	// DiscoverAIProviderModels 读取模型服务实例当前可用的模型目录。
	//cervi:route POST /settings/model-services/discover-models
	DiscoverAIProviderModels(context.Context, RequestMeta, AIProviderConnectionInput) (AIProviderModelList, error)
	// TestAIProviderConnection 测试模型服务供应商草稿配置。
	//cervi:route POST /settings/model-services/test
	TestAIProviderConnection(context.Context, RequestMeta, AIProviderConnectionInput) error
	// CreateAIProvider 创建模型服务供应商。
	//cervi:route POST /settings/model-services status=201
	CreateAIProvider(context.Context, RequestMeta, AIProviderInput) (AIProvider, error)
	// UpdateAIProvider 修改模型服务供应商。
	//cervi:route PUT /settings/model-services/:providerID
	UpdateAIProvider(context.Context, RequestMeta, string, AIProviderUpdateInput) (AIProvider, error)
	// DeleteAIProvider 删除模型服务供应商。
	//cervi:route DELETE /settings/model-services/:providerID
	DeleteAIProvider(context.Context, RequestMeta, string) error
	// ListMCPServers 返回当前企业配置的 MCP 服务。
	//cervi:route GET /settings/mcp-servers
	ListMCPServers(context.Context, RequestMeta) (MCPServerList, error)
	// GetMCPServer 返回当前企业中的 MCP 服务详情。
	//cervi:route GET /settings/mcp-servers/:mcpServerID
	GetMCPServer(context.Context, RequestMeta, string) (MCPServer, error)
	// TestMCPServerConnection 测试 MCP 草稿连接配置。
	//cervi:route POST /settings/mcp-servers/test-connection
	TestMCPServerConnection(context.Context, RequestMeta, MCPServerConnectionInput) error
	// TestSavedMCPServerConnection 测试已保存的 MCP 服务。
	//cervi:route POST /settings/mcp-servers/:mcpServerID/test-connection
	TestSavedMCPServerConnection(context.Context, RequestMeta, string) error
	// RefreshMCPServerTools 提交当前企业的 MCP 工具更新任务。
	//cervi:route POST /settings/mcp-servers/refresh-tools
	RefreshMCPServerTools(context.Context, RequestMeta) error

	// CreateMCPServer 创建 MCP 服务。
	//cervi:route POST /settings/mcp-servers status=201
	CreateMCPServer(context.Context, RequestMeta, MCPServerInput) (MCPServer, error)
	// UpdateMCPServer 修改 MCP 服务。
	//cervi:route PUT /settings/mcp-servers/:mcpServerID
	UpdateMCPServer(context.Context, RequestMeta, string, MCPServerInput) (MCPServer, error)
	// DeleteMCPServer 删除 MCP 服务。
	//cervi:route DELETE /settings/mcp-servers/:mcpServerID
	DeleteMCPServer(context.Context, RequestMeta, string) error
	// UpdateOrganization 修改当前企业通用设置。
	//cervi:route PUT /settings/organization
	UpdateOrganization(context.Context, RequestMeta, OrganizationInput) (Organization, error)
	// GetBusinessHours 读取当前企业的客服工作时间。
	//cervi:route GET /settings/customer-service/business-hours
	GetBusinessHours(context.Context, RequestMeta) (BusinessHours, error)
	// UpdateBusinessHours 修改当前企业的客服工作时间。
	//cervi:route PUT /settings/customer-service/business-hours
	UpdateBusinessHours(context.Context, RequestMeta, BusinessHours) (BusinessHours, error)
	// GetServiceTimeouts 读取当前企业的客服超时时长。
	//cervi:route GET /settings/customer-service/timeouts
	GetServiceTimeouts(context.Context, RequestMeta) (ServiceTimeouts, error)
	// UpdateServiceTimeouts 修改当前企业的客服超时时长。
	//cervi:route PUT /settings/customer-service/timeouts
	UpdateServiceTimeouts(context.Context, RequestMeta, ServiceTimeouts) (ServiceTimeouts, error)

	// RegisterDevice 注册当前用户的本机设备。
	//cervi:route POST /devices
	RegisterDevice(context.Context, RequestMeta, DeviceRegistrationInput) (Device, error)
	// ListDevices 返回当前用户已注册的设备。
	//cervi:route GET /devices
	ListDevices(context.Context, RequestMeta) (DeviceList, error)
	// RevokeDevice 撤销当前用户的设备。
	//cervi:route DELETE /devices/:deviceID
	RevokeDevice(context.Context, RequestMeta, string) error
	// RegisterDeviceWorkspace 在当前用户的设备上注册工作区。
	//cervi:route POST /devices/:deviceID/workspaces status=201
	RegisterDeviceWorkspace(context.Context, RequestMeta, string, DeviceWorkspaceInput) (DeviceWorkspace, error)
	// ListDeviceWorkspaces 返回当前用户设备上的工作区。
	//cervi:route GET /devices/:deviceID/workspaces
	ListDeviceWorkspaces(context.Context, RequestMeta, string) (DeviceWorkspaceList, error)
	// GetConversationDeviceBinding 返回会话绑定的设备与工作区。
	//cervi:route GET /conversations/:conversationID/device-binding
	GetConversationDeviceBinding(context.Context, RequestMeta, string) (ConversationDeviceBinding, error)
	// BindConversationDevice 把 AI 单聊绑定到本人设备上的工作区。
	//cervi:route PUT /conversations/:conversationID/device-binding
	BindConversationDevice(context.Context, RequestMeta, string, ConversationDeviceBindingInput) (ConversationDeviceBinding, error)
	// UnbindConversationDevice 解除 AI 单聊的设备绑定。
	//cervi:route DELETE /conversations/:conversationID/device-binding
	UnbindConversationDevice(context.Context, RequestMeta, string) error
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

// RealtimeConnector 由持有企业服务器实时事件流的原生端后端实现。
type RealtimeConnector interface {
	ConnectRealtime(context.Context, RequestMeta) (RealtimeConnection, error)
	DisconnectRealtime(context.Context, RequestMeta, string) error
	ConnectAgentRunStream(context.Context, RequestMeta, string) (RealtimeConnection, error)
	DisconnectAgentRunStream(context.Context, RequestMeta, string) error
}

// ImageSelector 由支持原生文件对话框的平台实现。
type ImageSelector interface {
	SelectImage(context.Context, RequestMeta) (ImageFile, error)
}

// ConversationWindowOpener 由支持多窗口的平台实现，在独立窗口打开指定会话。
type ConversationWindowOpener interface {
	OpenConversationWindow(context.Context, RequestMeta, ConversationWindowInput) error
}

// LocalDeviceReporter 由把本机注册为设备的原生端实现。
type LocalDeviceReporter interface {
	CurrentDevice(context.Context, RequestMeta) (LocalDevice, error)
}

// LocalWorkspaceManager 由支持本机 Agent 工作区的原生端实现。
type LocalWorkspaceManager interface {
	// AddLocalWorkspace 让用户选择本机目录并注册为本设备的工作区，用户取消选择时返回空工作区编号。
	AddLocalWorkspace(context.Context, RequestMeta) (DeviceWorkspace, error)
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
