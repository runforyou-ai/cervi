/** 集中定义 TanStack Query 查询 key 工厂。 */

/** 参与 key 组成的列表查询参数对象。 */
type KeyParameters = Record<string, unknown>

/** 生成「前缀 + 可选参数」的列表 key，不带参数时作为失效用前缀。 */
function listKey(prefix: string, parameters?: KeyParameters) {
  return parameters === undefined ? [prefix] : [prefix, parameters]
}

/** 生成「前缀 + 可选 ID」的单条数据 key，不带 ID 时作为失效用前缀。 */
function itemKey(prefix: string, id?: string) {
  return id === undefined ? [prefix] : [prefix, id]
}

/** 生成「前缀 + 可选归属 ID + 可选参数」的子列表 key，逐级省略时作为失效用前缀。 */
function scopedListKey(
  prefix: string,
  scopeId?: string,
  parameters?: KeyParameters,
) {
  if (scopeId === undefined) return [prefix]
  return parameters === undefined ? [prefix, scopeId] : [prefix, scopeId, parameters]
}

export const resourceKeys = {
  /** 当前登录身份、所属企业和用户偏好。 */
  identity: () => ["identity"],
  /** 客户会话的客户身份与当前周期访客上下文，参数为会话最新消息编号。 */
  customerProfile: (conversationId?: string, parameters?: KeyParameters) =>
    scopedListKey("customer-profile", conversationId, parameters),
  /** 客户会话当前客服周期的业务查询记录，随会话内容变化重读。 */
  customerBusinessQueries: (conversationId?: string) => itemKey("customer-business-queries", conversationId),
  /** 客户会话的交接摘要与客户历史周期小结，随会话内容变化重读。 */
  customerServiceSummaries: (conversationId?: string) => itemKey("customer-service-summaries", conversationId),
  /** 按会话编号读取的独立摘要。 */
  conversationSummary: (conversationId?: string) => itemKey("conversation-summary", conversationId),
  /** 指定会话的列表资格。 */
  inboxConversations: (parameters?: KeyParameters) => listKey("inbox-conversations", parameters),
  /** 当前窗口内消息的引用状态。 */
  conversationMessageReferences: (conversationId?: string, messageIds?: string) => scopedListKey("conversation-message-references", conversationId, messageIds === undefined ? undefined : { messageIds }),
  /** 当前会话窗口的外部投递状态。 */
  customerDeliveries: (conversationId?: string, messageIds?: string) =>
    scopedListKey("customer-deliveries", conversationId, messageIds === undefined ? undefined : { messageIds }),
  /** 收件箱数据。 */
  inbox: (parameters?: KeyParameters) => listKey("inbox", parameters),
  /** 当前用户的聊天提醒未读数、待处理的服务会话数与应用角标数。 */
  inboxAttention: (parameters?: KeyParameters) => listKey("inbox-attention", parameters),
  /** 原查询位置及前后窗口大小限定的会话邻域。 */
  inboxContext: (parameters?: KeyParameters) => listKey("inbox-context", parameters),
  /** 已加载双向边界限定的完整列表窗口。 */
  inboxWindow: (parameters?: KeyParameters) => listKey("inbox-window", parameters),
  /** 收件箱检索结果，参数包含检索文本与范围。 */
  inboxSearch: (parameters?: KeyParameters) => listKey("inbox-search", parameters),
  /** 本机最近打开会话的摘要，参数包含会话编号和列表筛选。 */
  recentConversations: (parameters?: KeyParameters) => listKey("recent-conversations", parameters),
  /** 客服筛选候选。 */
  customerServiceAssignees: () => ["customer-service-assignees"],
  /** 客服队列团队。 */
  serviceQueueTeams: () => ["service-queue-teams"],
  /** 渠道筛选候选。 */
  inboxChannels: () => ["inbox-channels"],
  /** 客服回复的译文与回译预览，参数为待翻译的回复。 */
  customerReplyTranslation: (conversationId: string, parameters?: KeyParameters) =>
    scopedListKey("customer-reply-translation", conversationId, parameters),
  /** 当前成员在客户会话中的翻译状态。 */
  conversationTranslation: (conversationId?: string) => itemKey("conversation-translation", conversationId),
  /** 一条客户会话消息面向指定语言的译文。 */
  messageTranslation: (conversationId?: string, parameters?: { messageId: string; language: string }) =>
    scopedListKey("message-translation", conversationId, parameters),
  /** 附件下载与图片读取地址。 */
  attachmentDownload: (conversationId: string, messageId?: string) => messageId === undefined
    ? ["attachment-download", conversationId] as const
    : ["attachment-download", conversationId, messageId] as const,
  /** 会话消息窗口的重读入口，参数区分同一会话的各个页面实例。 */
  conversationMessages: (conversationId?: string, parameters?: KeyParameters) =>
    scopedListKey("conversation-messages", conversationId, parameters),
  /** 成员消息最新页、前后分页与首尾游标限定的窗口范围。 */
  conversationMessagePage: (
    conversationId?: string,
    parameters?: KeyParameters,
  ) => scopedListKey("conversation-message-pages", conversationId, parameters),
  /** 目标消息上下文。 */
  conversationMessageContext: (conversationId: string, messageId?: string) =>
    scopedListKey(
      "conversation-message-context",
      conversationId,
      messageId ? { messageId } : undefined,
    ),
  /** 按运行编号读取的 AI 运行过程详情。 */
  agentRunProcess: (runId?: string) => itemKey("agent-run-process", runId),
  /** 当前群聊的提及进度。 */
  conversationNavigation: (conversationId?: string) =>
    itemKey("conversation-navigation", conversationId),
  /** 开始一轮导航时读取的提及队列。 */
  conversationMentions: (conversationId?: string) =>
    itemKey("conversation-mentions", conversationId),
  /** 当前成员与目标身份的已有单聊。 */
  directConversation: (identityId?: string) =>
    itemKey("direct-conversation", identityId),
  /** 可用于 AI 写回复的 AI 员工。 */
  customerReplyAgents: () => ["customer-reply-agents"],
  /** 客户会话的 Copilot 线程列表。 */
  customerCopilotThreads: (conversationId?: string) => itemKey("customer-copilot-threads", conversationId),
  /** 客户会话 AI 写回复的候选，不带参数时作为该会话全部候选的失效前缀。 */
  customerReplySuggestions: (conversationId: string, parameters?: KeyParameters) =>
    parameters === undefined
      ? ["customer-reply-suggestions", conversationId]
      : ["customer-reply-suggestions", conversationId, parameters],
  /** 发起内部会话时使用的成员候选项。 */
  memberOptions: () => ["member-options"],
  /** 可发起单聊的对象，含本人名下的助理。 */
  chatTargets: () => ["chat-targets"],
  /** 单个群聊资料和当前成员。 */
  groupConversation: (conversationId?: string) =>
    itemKey("group-conversation", conversationId),
  /** 消息渠道列表。 */
  messageChannels: () => ["message-channels"],
  /** 单个消息渠道，按类型与 ID 标识。 */
  messageChannel: (type: string, id: string) => ["message-channel", type, id],
  /** 跨业务域复用的渠道选项。 */
  channelOptions: () => ["channel-options"],
  /** 渠道接待设置选项。 */
  channelReceptionOptions: () => ["channel-reception-options"],
  /** 网站渠道访问地址。 */
  websiteChannelOrigin: () => ["website-channel-origin"],
  /** AI 模型服务商列表。 */
  aiProviders: () => ["ai-providers"],
  /** 单个 AI 模型服务商。 */
  aiProvider: (id?: string) => itemKey("ai-provider", id),
  /** 智能体可选模型选项。 */
  agentModelOptions: () => ["agent-model-options"],
  /** AI 员工配置使用的 MCP 服务摘要。 */
  agentMCPServerOptions: () => ["agent-mcp-server-options"],
  /** MCP 服务列表。 */
  mcpServers: () => ["mcp-servers"],
  /** 单个 MCP 服务。 */
  mcpServer: (id?: string) => itemKey("mcp-server", id),
  /** 当前用户已注册的设备列表。 */
  devices: () => ["devices"],
  /** 本机在当前企业服务器上的设备注册状态。 */
  currentDevice: () => ["current-device"],
  /** 当前企业的客服工作时间。 */
  businessHours: () => ["business-hours"],
  /** 当前企业的客服超时时长。 */
  serviceTimeouts: () => ["service-timeouts"],
  /** 当前企业的会话小结设置。 */
  serviceSummarySettings: () => ["service-summary-settings"],
  /** 企业翻译设置。 */
  translationSettings: () => ["translation-settings"],
  /** 企业联网搜索设置。 */
  webSearchSettings: () => ["web-search-settings"],
  /** 当前企业的客户身份密钥。 */
  customerIdentitySecret: () => ["customer-identity-secret"],
  /** 当前企业的咨询分类目录。 */
  serviceCategories: () => ["service-categories"],
  /** AI 表现报表概览，参数包含统计天数与渠道。 */
  aiPerformanceReport: (parameters?: KeyParameters) => listKey("ai-performance-report", parameters),
  /** AI 表现按维度拆分，参数包含统计天数、渠道、维度与分页。 */
  aiPerformanceBreakdowns: (parameters?: KeyParameters) => listKey("ai-performance-breakdowns", parameters),
  /** 待补知识清单，参数包含渠道、处理状态与分页。 */
  knowledgeGaps: (parameters?: KeyParameters) => listKey("knowledge-gaps", parameters),
  /** 单条待补知识详情。 */
  knowledgeGap: (id?: string) => itemKey("knowledge-gap", id),
  /** 待补知识的问题在指定知识库中召回的相似问答。 */
  knowledgeGapSimilarQA: (gapId: string, knowledgeBaseId: string, query: string) => ["knowledge-gap-similar-qa", gapId, knowledgeBaseId, query],
  /** 当前客户端连接的企业服务器地址。 */
  serverURL: () => ["server-url"],
  /** 知识库列表。 */
  knowledgeBases: () => ["knowledge-bases"],
  /** 单个知识库。 */
  knowledgeBase: (id?: string) => itemKey("knowledge-base", id),
  /** 当前配置版本绑定指定知识库的 AI 员工。 */
  knowledgeBaseAgents: (knowledgeBaseId?: string) => itemKey("knowledge-base-agents", knowledgeBaseId),
  /** 指定知识库的问答列表。 */
  knowledgeQAEntries: (knowledgeBaseId?: string, parameters?: KeyParameters) =>
    scopedListKey("knowledge-qa-entries", knowledgeBaseId, parameters),
  /** 指定知识库中的完整问答。 */
  knowledgeQAEntry: (knowledgeBaseId: string, entryId?: string) =>
    entryId === undefined
      ? ["knowledge-qa-entry", knowledgeBaseId]
      : ["knowledge-qa-entry", knowledgeBaseId, entryId],
  /** 指定知识库的文档列表。 */
  knowledgeDocuments: (baseId?: string, parameters?: KeyParameters) => scopedListKey("knowledge-documents", baseId, parameters),
  /** 单个文档详情。 */
  knowledgeDocument: (baseId: string, documentId: string) => ["knowledge-document", baseId, documentId],
  /** 在线文档正文或网页抓取快照。 */
  knowledgeDocumentContent: (baseId: string, documentId: string) => ["knowledge-document-content", baseId, documentId],
  /** 指定文档批次和首次阅读锚点的分段列表。 */
  knowledgeDocumentSegments: (baseId: string, documentId: string, batchId: string, anchorSegmentId = "") => ["knowledge-document-segments", baseId, documentId, batchId, anchorSegmentId],
  /** 文档原件的客户端预览。 */
  knowledgeDocumentFile: (baseId: string, documentId: string) => ["knowledge-document-file", baseId, documentId],
  /** 角色列表。 */
  roles: () => ["roles"],
  /** 单个角色。 */
  role: (id?: string) => itemKey("role", id),
  /** 角色配置使用的全部真人和 AI 员工。 */
  roleMembers: () => ["role-members"],
  /** 成员列表，可带筛选分页参数。 */
  users: (parameters?: KeyParameters) => listKey("users", parameters),
  /** 全量成员列表。 */
  usersAll: () => ["users", "all"],
  /** 单个成员。 */
  user: (id?: string) => itemKey("user", id),
  /** 智能体列表，可带筛选分页参数。 */
  agents: (parameters?: KeyParameters) => listKey("agents", parameters),
  /** 单个智能体。 */
  agent: (id?: string) => itemKey("agent", id),
  /** 当前成员名下的助理列表。 */
  assistants: () => ["assistants"],
  /** 当前成员名下的单个助理。 */
  assistant: (id?: string) => itemKey("assistant", id),
  /** 指定成员名下的助理列表。 */
  memberAssistants: (userId?: string) => itemKey("member-assistants", userId),
  /** 团队列表，可带分页参数。 */
  teams: (parameters?: KeyParameters) => listKey("teams", parameters),
  /** 单个团队。 */
  team: (id?: string) => itemKey("team", id),
  /** 团队成员列表，按团队 ID 与筛选分页参数标识。 */
  teamMembers: (teamId?: string, parameters?: KeyParameters) =>
    scopedListKey("team-members", teamId, parameters),
  /** 团队候选成员列表，按团队 ID 与筛选分页参数标识。 */
  teamMemberCandidates: (teamId?: string, parameters?: KeyParameters) =>
    scopedListKey("team-member-candidates", teamId, parameters),
  /** 外部联系人列表，可带回收站与筛选分页参数。 */
  contacts: (parameters?: KeyParameters) => listKey("contacts", parameters),
  /** 单个外部联系人。 */
  contact: (id?: string) => itemKey("contact", id),
}
