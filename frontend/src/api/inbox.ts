/** 成员收件箱与会话消息调用归一化。 */
import {
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
  ListConversationMessages,
  MarkConversationRead,
  ListCustomerServiceAssignees,
  LoadInbox,
  ReopenServiceSession,
  RemoveGroupConversationMember,
  SendCustomerTextMessage,
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
  ConversationMessageList,
  ConversationMessageListInput,
  ConversationNotificationSettings,
  ConversationNotificationSettingsInput,
  ConversationUnreadMarkInput,
  CustomerTextMessageInput,
  DirectInboxConversation,
  FirstDirectTextMessageInput,
  DirectTextMessageInput,
  GroupConversation,
  GroupConversationInput,
  GroupConversationLeaveInput,
  GroupConversationMemberInput,
  GroupConversationMembersInput,
  GroupConversationOwnerInput,
  GroupConversationProfileInput,
  GroupInboxConversation,
  GroupParticipant,
  GroupTextMessageInput,
  Inbox,
  InboxConversation,
  LoadInboxInput,
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
import { asList } from "@/api/normalize"

export type InboxData = Omit<Inbox, "conversations"> & {
  conversations: InboxConversation[]
}

export type ConversationMessageListData = Omit<
  ConversationMessageList,
  "messages"
> & {
  messages: ConversationMessageData[]
}

export type ConversationMessageData = Omit<ConversationMessage, "mentions"> & {
  mentions: NonNullable<ConversationMessage["mentions"]>
}

export type CustomerInboxConversationData = InboxConversation & {
  type: ConversationType.ConversationTypeCustomer
  customer: CustomerInboxConversation
  direct: null
  group: null
}

export type DirectInboxConversationData = InboxConversation & {
  type: ConversationType.ConversationTypeDirect
  customer: null
  direct: DirectInboxConversation
  group: null
}

export type GroupInboxConversationData = InboxConversation & {
  type: ConversationType.ConversationTypeGroup
  customer: null
  direct: null
  group: GroupInboxConversation
}

export type GroupConversationData = Omit<GroupConversation, "participants"> & {
  participants: GroupParticipant[]
}

export type GroupTextMessageDataInput = Omit<
  GroupTextMessageInput,
  "mentionAll"
>

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
const removeGroupConversationMemberBound = bind(
  RemoveGroupConversationMember,
)
const transferGroupConversationOwnerBound = bind(
  TransferGroupConversationOwner,
)
const leaveGroupConversationBound = bind(LeaveGroupConversation)
const sendGroupTextMessageBound = bind(SendGroupTextMessage)
const listCustomerServiceAssigneesBound = bind(ListCustomerServiceAssignees)
const claimServiceSessionBound = bind(ClaimServiceSession)
const transferServiceSessionBound = bind(TransferServiceSession)
const closeServiceSessionBound = bind(CloseServiceSession)
const reopenServiceSessionBound = bind(ReopenServiceSession)

export type LoadInboxQuery = Partial<LoadInboxInput>

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

/** 归一化消息中的提醒和系统事件成员列表。 */
function normalizeConversationMessage(
  message: ConversationMessage,
): ConversationMessageData {
  return {
    ...message,
    mentions: asList(message.mentions),
    systemEvent: message.systemEvent
      ? {
          ...message.systemEvent,
          targets: asList(message.systemEvent.targets),
        }
      : null,
  }
}

/** 判断统一收件箱项是否为结构完整的客户会话。 */
export function isCustomerInboxConversation(
  conversation: InboxConversation,
): conversation is CustomerInboxConversationData {
  return (
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
    conversation.type === ConversationType.ConversationTypeGroup &&
    conversation.customer === null &&
    conversation.direct === null &&
    conversation.group !== null
  )
}

/** 读取成员统一收件箱会话列表。 */
export async function loadInbox(
  query: LoadInboxQuery = {},
): Promise<InboxData> {
  const inbox = await loadInboxBound({
    scope: query.scope ?? InboxScope.InboxScopeAll,
    customerView:
      query.customerView ?? CustomerInboxView.CustomerInboxViewQueue,
    assigneeIdentityId: query.assigneeIdentityId ?? "",
  })
  return {
    ...inbox,
    conversations: asList(inbox.conversations),
  }
}

/** 读取有效真人和 AI 客服筛选项。 */
export async function listCustomerServiceAssignees() {
  const output = await listCustomerServiceAssigneesBound()
  return asList(output.assignees)
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
export async function listConversationMessages(
  conversationID: string,
  input: ConversationMessageListInput = { before: "", after: "" },
  signal?: AbortSignal,
): Promise<ConversationMessageListData> {
  const result = await listConversationMessagesBound(
    conversationID,
    input,
    signal,
  )
  return {
    ...result,
    messages: asList(result.messages).map(normalizeConversationMessage),
  }
}

/** 单调推进当前用户的原生会话已读水位。 */
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
export async function sendCustomerTextMessage(
  conversationID: string,
  input: CustomerTextMessageInput,
) {
  const message = await sendCustomerTextMessageBound(conversationID, input)
  return normalizeConversationMessage(message)
}

/** 发送首条单聊消息并返回最终会话。 */
export async function sendFirstDirectTextMessage(
  input: FirstDirectTextMessageInput,
) {
  const result = await sendFirstDirectTextMessageBound(input)
  return {
    ...result,
    conversation: result.conversation as DirectInboxConversationData,
    message: normalizeConversationMessage(result.message),
  }
}

/** 按目标身份查找当前成员的活跃单聊。 */
export async function findDirectConversation(targetIdentityID: string) {
  const result = await findDirectConversationBound(targetIdentityID)
  return result.conversation as DirectInboxConversationData | null
}

/** 发送企业成员内部单聊文本消息。 */
export async function sendDirectTextMessage(
  conversationID: string,
  input: DirectTextMessageInput,
) {
  const message = await sendDirectTextMessageBound(conversationID, input)
  return normalizeConversationMessage(message)
}

/** 创建企业内部群聊。 */
export function createGroupConversation(input: GroupConversationInput) {
  return createGroupConversationBound(input)
}

/** 读取企业内部群聊资料和当前成员。 */
export async function getGroupConversation(
  conversationID: string,
): Promise<GroupConversationData> {
  const result = await getGroupConversationBound(conversationID)
  return { ...result, participants: asList(result.participants) }
}

/** 修改企业内部群聊资料。 */
export async function updateGroupConversation(
  conversationID: string,
  input: GroupConversationProfileInput,
): Promise<GroupConversationData> {
  const result = await updateGroupConversationBound(conversationID, input)
  return { ...result, participants: asList(result.participants) }
}

/** 批量增加企业内部群聊成员。 */
export async function addGroupConversationMembers(
  conversationID: string,
  input: GroupConversationMembersInput,
): Promise<GroupConversationData> {
  const result = await addGroupConversationMembersBound(
    conversationID,
    input,
  )
  return { ...result, participants: asList(result.participants) }
}

/** 移除企业内部群聊成员。 */
export async function removeGroupConversationMember(
  conversationID: string,
  input: GroupConversationMemberInput,
): Promise<GroupConversationData> {
  const result = await removeGroupConversationMemberBound(
    conversationID,
    input,
  )
  return { ...result, participants: asList(result.participants) }
}

/** 转让企业内部群聊群主。 */
export async function transferGroupConversationOwner(
  conversationID: string,
  input: GroupConversationOwnerInput,
): Promise<GroupConversationData> {
  const result = await transferGroupConversationOwnerBound(
    conversationID,
    input,
  )
  return { ...result, participants: asList(result.participants) }
}

/** 退出企业内部群聊。 */
export function leaveGroupConversation(
  conversationID: string,
  input: GroupConversationLeaveInput,
) {
  return leaveGroupConversationBound(conversationID, input)
}

/** 发送企业内部群聊文本消息。 */
export async function sendGroupTextMessage(
  conversationID: string,
  input: GroupTextMessageDataInput,
) {
  const message = await sendGroupTextMessageBound(conversationID, {
    ...input,
    mentionAll: false,
  })
  return normalizeConversationMessage(message)
}

/** 读取目标消息周围的连续上下文。 */
export async function getConversationMessageContext(
  conversationID: string,
  messageID: string,
  signal?: AbortSignal,
): Promise<ConversationMessageListData> {
  const result = await getConversationMessageContextBound(
    conversationID,
    messageID,
    signal,
  )
  return {
    ...result,
    messages: asList(result.messages).map(normalizeConversationMessage),
  }
}

/** 读取群聊待查看数量及最新可见消息。 */
export function getConversationNavigationState(
  conversationID: string,
  signal?: AbortSignal,
) {
  return getConversationNavigationStateBound(conversationID, signal)
}

/** 获取本轮固定的提及目标列表。 */
export async function listPendingConversationMentions(
  conversationID: string,
  signal?: AbortSignal,
) {
  const result = await listPendingConversationMentionsBound(
    conversationID,
    signal,
  )
  return { ...result, messageIds: asList(result.messageIds) }
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
