/** 成员收件箱与会话消息调用。 */
import {
  ListCustomerMessageDeliveries,
  ListConversationMessageReferences,
  ResolveCustomerMessageDelivery,
  AddGroupConversationMembers,
  ClaimServiceSession,
  CloseServiceSession,
  CreateGroupConversation,
  FindDirectConversation,
  GetGroupConversation,
  GetConversationMessageContext,
  GetConversationNavigationState,
  ListPendingConversationMentions,
  MarkConversationMentionReviewed,
  LeaveGroupConversation,
  DissolveGroupConversation,
  ListConversationMessages,
  MarkConversationRead,
  ListCustomerServiceAssignees,
  LoadInbox,
  GetInboxContext,
  ReadInboxWindow,
  GetInboxConversation,
  ReadInboxConversations,
  ReopenServiceSession,
  RemoveGroupConversationMember,
  SendCustomerTextMessage,
  SendAttachmentMessage,
  SendAttachmentBatch,
  UpdateAttachmentUploads,
  ListAttachmentStates,
  CompleteAttachmentUpload,
  GetAttachmentDownload,
  SendFirstAgentTextMessage,
  SendAgentTextMessage,
  StopAgentReply,
  SendFirstDirectTextMessage,
  SendDirectTextMessage,
  SendGroupTextMessage,
  TransferServiceSession,
  TransferGroupConversationOwner,
  UpdateGroupConversation,
  UpdateConversationNotificationSettings,
  UpdateConversationUnreadMark,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  CustomerInboxConversation,
  ConversationMessage,
  ConversationAgentProcess,
  ConversationMessageList,
  ConversationMessageListInput,
  ConversationNotificationSettings,
  ConversationNotificationSettingsInput,
  ConversationUnreadMarkInput,
  CustomerTextMessageInput,
  DirectInboxConversation,
  FirstAgentTextMessageInput,
  AgentTextMessageInput,
  AgentInboxConversation,
  FirstDirectTextMessageInput,
  DirectTextMessageInput,
  GroupConversation,
  GroupConversationInput,
  GroupConversationMemberInput,
  GroupConversationMembersInput,
  GroupConversationOwnerInput,
  GroupConversationProfileInput,
  GroupInboxConversation,
  GroupTextMessageInput,
  Inbox,
  ReadInboxConversationsInput,
  InboxConversation,
  LoadInboxInput,
  InboxContextInput,
  InboxWindowInput,
  InboxQuery,
  MarkConversationReadInput,
  TransferServiceSessionInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import {
  ConversationType,
  CustomerInboxView,
  InboxScope,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { enqueueConversationUnreadChange } from "@/api/conversation-read-queue"
import type { NonNullArrays } from "@/api/normalize"

export type InboxData = NonNullArrays<Inbox>

export type ConversationMessageListData = NonNullArrays<ConversationMessageList>

export type ConversationAgentProcessData =
  NonNullArrays<ConversationAgentProcess>

export type ConversationMessageData = NonNullArrays<ConversationMessage>

export type CustomerInboxConversationData = NonNullArrays<InboxConversation> & {
  type: ConversationType.ConversationTypeCustomer
  customer: CustomerInboxConversation
  direct: null
  group: null
  agent: null
}

export type DirectInboxConversationData = NonNullArrays<InboxConversation> & {
  type: ConversationType.ConversationTypeDirect
  customer: null
  direct: DirectInboxConversation
  group: null
  agent: null
}

export type GroupInboxConversationData = NonNullArrays<InboxConversation> & {
  type: ConversationType.ConversationTypeGroup
  customer: null
  direct: null
  group: GroupInboxConversation
  agent: null
}

export type AgentInboxConversationData = NonNullArrays<InboxConversation> & {
  type: ConversationType.ConversationTypeAgent
  customer: null
  direct: null
  group: null
  agent: AgentInboxConversation
}

export type GroupConversationData = NonNullArrays<GroupConversation>

const loadInboxBound = bind(LoadInbox)
const listConversationMessagesBound = bind(ListConversationMessages)
const markConversationReadBound = bind(MarkConversationRead)
const getConversationMessageContextBound = bind(GetConversationMessageContext)
const getConversationNavigationStateBound = bind(GetConversationNavigationState)
const listPendingConversationMentionsBound = bind(
  ListPendingConversationMentions,
)
const markConversationMentionReviewedBound = bind(
  MarkConversationMentionReviewed,
)
const sendCustomerTextMessageBound = bind(SendCustomerTextMessage)
const sendFirstAgentTextMessageBound = bind(SendFirstAgentTextMessage)
const sendAgentTextMessageBound = bind(SendAgentTextMessage)
const sendFirstDirectTextMessageBound = bind(SendFirstDirectTextMessage)
const findDirectConversationBound = bind(FindDirectConversation)
const sendDirectTextMessageBound = bind(SendDirectTextMessage)
const createGroupConversationBound = bind(CreateGroupConversation)
const getGroupConversationBound = bind(GetGroupConversation)
const updateGroupConversationBound = bind(UpdateGroupConversation)
const updateConversationNotificationSettingsBound = bind(
  UpdateConversationNotificationSettings,
)
const addGroupConversationMembersBound = bind(AddGroupConversationMembers)
const removeGroupConversationMemberBound = bind(RemoveGroupConversationMember)
const transferGroupConversationOwnerBound = bind(TransferGroupConversationOwner)
const leaveGroupConversationBound = bind(LeaveGroupConversation)
const dissolveGroupConversationBound = bind(DissolveGroupConversation)
const sendGroupTextMessageBound = bind(SendGroupTextMessage)
const listCustomerServiceAssigneesBound = bind(ListCustomerServiceAssignees)
const claimServiceSessionBound = bind(ClaimServiceSession)
const transferServiceSessionBound = bind(TransferServiceSession)
const closeServiceSessionBound = bind(CloseServiceSession)
const reopenServiceSessionBound = bind(ReopenServiceSession)

export type LoadInboxQuery = Partial<InboxQuery>

const updateConversationUnreadMarkBound = bind(UpdateConversationUnreadMark)

/** 保存独立于阅读水位的个人未读标记。 */
export function updateConversationUnreadMark(
  conversationID: string,
  input: ConversationUnreadMarkInput,
) {
  return enqueueConversationUnreadChange(conversationID, () =>
    updateConversationUnreadMarkBound(conversationID, input),
  )
}

/** 保存当前用户的原生会话提醒设置。 */
export function updateConversationNotificationSettings(
  conversationID: string,
  input: ConversationNotificationSettingsInput,
): Promise<ConversationNotificationSettings> {
  return updateConversationNotificationSettingsBound(conversationID, input)
}

/** 判断统一收件箱项是否为结构完整的客户会话。 */
export function isCustomerInboxConversation(
  conversation: InboxConversation,
): conversation is CustomerInboxConversationData {
  return (
    conversation.agent === null &&
    conversation.type === ConversationType.ConversationTypeCustomer &&
    conversation.customer !== null &&
    conversation.direct === null &&
    conversation.group === null
  )
}

/** 判断统一收件箱项是否为结构完整的内部单聊。 */
export function isDirectInboxConversation(
  conversation: InboxConversation,
): conversation is DirectInboxConversationData {
  return (
    conversation.agent === null &&
    conversation.type === ConversationType.ConversationTypeDirect &&
    conversation.customer === null &&
    conversation.direct !== null &&
    conversation.group === null
  )
}

/** 判断统一收件箱项是否为结构完整的企业群聊。 */
export function isGroupInboxConversation(
  conversation: InboxConversation,
): conversation is GroupInboxConversationData {
  return (
    conversation.agent === null &&
    conversation.type === ConversationType.ConversationTypeGroup &&
    conversation.customer === null &&
    conversation.direct === null &&
    conversation.group !== null
  )
}

/** 读取成员统一收件箱会话列表。 */
export async function loadInbox(
  query: Partial<LoadInboxInput> = {},
): Promise<InboxData> {
  const inbox = await loadInboxBound({
    scope: query.scope ?? InboxScope.InboxScopeAll,
    customerView:
      query.customerView ?? CustomerInboxView.CustomerInboxViewQueue,
    assigneeIdentityId: query.assigneeIdentityId ?? "",
    cursor: query.cursor ?? "",
    beforeCursor: query.beforeCursor ?? "",
    limit: query.limit ?? 50,
  })
  return inbox
}

/** 读取有效真人和 AI 客服筛选项。 */
export async function listCustomerServiceAssignees() {
  const output = await listCustomerServiceAssigneesBound()
  return output.assignees
}

/** 领取或接管客户会话最新处理周期。 */
export function claimServiceSession(conversationId: string) {
  return claimServiceSessionBound(conversationId)
}

/** 把当前负责的处理周期转给另一位客服。 */
export function transferServiceSession(
  conversationId: string,
  input: TransferServiceSessionInput,
) {
  return transferServiceSessionBound(conversationId, input)
}

/** 关闭客户会话最新处理周期。 */
export function closeServiceSession(conversationId: string) {
  return closeServiceSessionBound(conversationId)
}

/** 重新打开客户会话并分配给当前身份。 */
export function reopenServiceSession(conversationId: string) {
  return reopenServiceSessionBound(conversationId)
}

/** 分页读取成员可见的会话消息。 */
export function listConversationMessages(
  conversationID: string,
  input: ConversationMessageListInput = { before: "", after: "" },
  signal?: AbortSignal,
) {
  return listConversationMessagesBound(conversationID, input, signal)
}

/** 单调推进当前用户的会话已读水位。 */
export function markConversationRead(
  conversationID: string,
  input: MarkConversationReadInput,
) {
  if (input.clearUnreadMark) {
    return enqueueConversationUnreadChange(conversationID, () =>
      markConversationReadBound(conversationID, input),
    )
  }
  return markConversationReadBound(conversationID, input)
}

/** 发送成员客户会话文本消息。 */
export function sendCustomerTextMessage(conversationID: string, input: CustomerTextMessageInput) {
  return sendCustomerTextMessageBound(conversationID, input)
}

/** 发送首条单聊消息并返回最终会话。 */
export async function sendFirstDirectTextMessage(
  input: FirstDirectTextMessageInput,
) {
  const result = await sendFirstDirectTextMessageBound(input)
  return {
    ...result,
    conversation: result.conversation as DirectInboxConversationData,
  }
}

/** 按目标身份查找当前成员的活跃单聊。 */
export async function findDirectConversation(targetIdentityID: string) {
  const result = await findDirectConversationBound(targetIdentityID)
  return result.conversation as DirectInboxConversationData | null
}

/** 发送企业成员内部单聊文本消息。 */
export function sendDirectTextMessage(conversationID: string, input: DirectTextMessageInput) {
  return sendDirectTextMessageBound(conversationID, input)
}

/** 创建企业内部群聊。 */
export function createGroupConversation(input: GroupConversationInput) {
  return createGroupConversationBound(input)
}

/** 读取企业内部群聊资料和当前成员。 */
export async function getGroupConversation(
  conversationID: string,
): Promise<GroupConversationData> {
  return getGroupConversationBound(conversationID)
}

/** 修改企业内部群聊资料。 */
export async function updateGroupConversation(
  conversationID: string,
  input: GroupConversationProfileInput,
): Promise<GroupConversationData> {
  return updateGroupConversationBound(conversationID, input)
}

/** 批量增加企业内部群聊成员。 */
export async function addGroupConversationMembers(
  conversationID: string,
  input: GroupConversationMembersInput,
): Promise<GroupConversationData> {
  return addGroupConversationMembersBound(conversationID, input)
}

/** 移除企业内部群聊成员。 */
export async function removeGroupConversationMember(
  conversationID: string,
  input: GroupConversationMemberInput,
): Promise<GroupConversationData> {
  return removeGroupConversationMemberBound(conversationID, input)
}

/** 转让企业内部群聊群主。 */
export async function transferGroupConversationOwner(
  conversationID: string,
  input: GroupConversationOwnerInput,
): Promise<GroupConversationData> {
  return transferGroupConversationOwnerBound(
    conversationID,
    input,
  )
}

/** 退出企业内部群聊。 */
export function leaveGroupConversation(
  conversationID: string,
) {
  return leaveGroupConversationBound(conversationID)
}

/** 解散群聊并归一化保留的成员列表。 */
export async function dissolveGroupConversation(conversationID: string) {
  return dissolveGroupConversationBound(conversationID)
}

/** 发送企业内部群聊文本消息。 */
export function sendGroupTextMessage(conversationID: string, input: GroupTextMessageInput) {
  return sendGroupTextMessageBound(conversationID, input)
}

/** 读取目标消息周围的连续上下文。 */
export function getConversationMessageContext(
  conversationID: string,
  messageID: string,
  signal?: AbortSignal,
) {
  return getConversationMessageContextBound(conversationID, messageID, signal)
}

/** 读取群聊待查看数量及最新可见消息。 */
export function getConversationNavigationState(
  conversationID: string,
  signal?: AbortSignal,
) {
  return getConversationNavigationStateBound(conversationID, signal)
}

/** 获取本轮固定的提及目标列表。 */
export function listPendingConversationMentions(
  conversationID: string,
  signal?: AbortSignal,
) {
  return listPendingConversationMentionsBound(conversationID, signal)
}

/** 确认一条实际查看的提及目标。 */
export function markConversationMentionReviewed(
  conversationID: string,
  messageID: string,
) {
  return markConversationMentionReviewedBound(conversationID, {
    messageId: messageID,
  })
}

/** 判断收件箱项是否为独立 AI 聊天。 */
export function isAgentInboxConversation(
  conversation: InboxConversation,
): conversation is AgentInboxConversationData {
  return (
    conversation.type === ConversationType.ConversationTypeAgent &&
    conversation.agent !== null &&
    conversation.direct === null &&
    conversation.customer === null &&
    conversation.group === null
  )
}

/** 确认 AI 草稿对应的会话并保存首条消息。 */
export async function sendFirstAgentTextMessage(
  input: FirstAgentTextMessageInput,
) {
  const result = await sendFirstAgentTextMessageBound(input)
  return {
    ...result,
    conversation: result.conversation as AgentInboxConversationData,
  }
}

/** 向指定 AI 会话发送成员消息。 */
export function sendAgentTextMessage(
  conversationID: string,
  input: AgentTextMessageInput,
) {
  return sendAgentTextMessageBound(conversationID, input)
}

/** 读取当前窗口的客户消息投递状态。 */
const listCustomerMessageDeliveriesBound = bind(ListCustomerMessageDeliveries)
/** 读取当前消息窗口的投递集合。 */
export function listCustomerMessageDeliveries(conversationID: string, messageIds: string) {
  return listCustomerMessageDeliveriesBound(conversationID, { messageIds })
}
/** 人工确认或重试一条客户消息投递。 */
export const resolveCustomerMessageDelivery = bind(ResolveCustomerMessageDelivery)

/** 发送附件消息。 */
export const sendAttachmentMessage = bind(SendAttachmentMessage)

/** 获取当前可见附件的下载请求。 */
export const getAttachmentDownload = bind(GetAttachmentDownload)

/** 保存一批按顺序发送的单聊附件和说明。 */
export const sendAttachmentBatch = bind(SendAttachmentBatch)
/** 更新上传状态、续期当前上传或取消未完成的附件。 */
export const updateAttachmentUploads = bind(UpdateAttachmentUploads)

/** 读取窗口内已经存在的附件消息状态。 */
export function listAttachmentStates(conversationID: string, messageIDs: string) {
  return bind(ListAttachmentStates)(conversationID, { messageIds: messageIDs })
}

/** 完成上传并激活原附件消息。 */
export const completeAttachmentUpload = bind(CompleteAttachmentUpload)

/** 停止指定 AI 回复并读取实际运行状态。 */
export const stopAgentReply = bind(StopAgentReply)

/** 独立读取当前用户可见的会话摘要。 */
export const getInboxConversation = bind(GetInboxConversation)

/** 批量核对指定会话的阅读和列表资格。 */
export function readInboxConversations(input: ReadInboxConversationsInput, signal?: AbortSignal) {
  return bind(ReadInboxConversations)(input, signal)
}

const listConversationMessageReferencesBound = bind(ListConversationMessageReferences)
/** 读取当前窗口消息的最新引用和回复可用状态。 */
export function listConversationMessageReferences(conversationID: string, messageIds: string) {
  return listConversationMessageReferencesBound(conversationID, { messageIds })
}

/** 读取会话原位置附近的列表窗口及当前资格。 */
export function getInboxContext(input: InboxContextInput) {
  return bind(GetInboxContext)(input)
}

/** 重读已加载首尾边界之间的完整列表范围。 */
export function readInboxWindow(input: InboxWindowInput) {
  return bind(ReadInboxWindow)(input)
}
