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
  /** 当前用户应用角标的提醒总数，合计内部会话提醒与处理中客户会话里提醒本人的未读。 */
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
  /** 业务系统列表。 */
  businessSystems: () => ["business-systems"],
  /** 单个业务系统。 */
  businessSystem: (id?: string) => itemKey("business-system", id),
  /** MCP 服务列表。 */
  mcpServers: () => ["mcp-servers"],
  /** 单个 MCP 服务。 */
  mcpServer: (id?: string) => itemKey("mcp-server", id),
  /** 知识库列表。 */
  knowledgeBases: () => ["knowledge-bases"],
  /** 单个知识库。 */
  knowledgeBase: (id?: string) => itemKey("knowledge-base", id),
  /** 当前配置版本绑定指定知识库的 AI 员工。 */
  knowledgeBaseAgents: (knowledgeBaseId?: string) => itemKey("knowledge-base-agents", knowledgeBaseId),
  /** 指定知识库及分组条件的问答列表。 */
  knowledgeQAEntries: (knowledgeBaseId?: string, parameters?: KeyParameters) =>
    scopedListKey("knowledge-qa-entries", knowledgeBaseId, parameters),
  /** 指定知识库中的完整问答。 */
  knowledgeQAEntry: (knowledgeBaseId: string, entryId?: string) =>
    entryId === undefined
      ? ["knowledge-qa-entry", knowledgeBaseId]
      : ["knowledge-qa-entry", knowledgeBaseId, entryId],
  /** 指定知识库及分组条件的文档列表。 */
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
  /** 团队列表，可带分页参数。 */
  teams: (parameters?: KeyParameters) => listKey("teams", parameters),
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
