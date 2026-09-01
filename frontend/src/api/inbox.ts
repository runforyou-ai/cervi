/** 成员收件箱与会话消息调用归一化。 */
import {
  ClaimServiceSession,
  CloseServiceSession,
  CreateGroupConversation,
  GetGroupConversation,
  ListConversationMessages,
  ListCustomerServiceAssignees,
  LoadInbox,
  ReopenServiceSession,
  SendCustomerTextMessage,
  SendDirectTextMessage,
  SendGroupTextMessage,
  StartDirectConversation,
  TransferServiceSession,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  CustomerInboxConversation,
  ConversationMessage,
  ConversationMessageList,
  ConversationMessageListInput,
  CustomerTextMessageInput,
  DirectInboxConversation,
  DirectConversationInput,
  DirectTextMessageInput,
  GroupConversation,
  GroupConversationInput,
  GroupInboxConversation,
  GroupParticipant,
  GroupTextMessageInput,
  Inbox,
  InboxConversation,
  LoadInboxInput,
  TransferServiceSessionInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import {
  ConversationType,
  CustomerInboxView,
  InboxScope,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type InboxData = Omit<Inbox, "conversations"> & {
  conversations: InboxConversation[]
}

export type ConversationMessageListData = Omit<
  ConversationMessageList,
  "messages"
> & {
  messages: ConversationMessage[]
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

const loadInboxBound = bind(LoadInbox)
const listConversationMessagesBound = bind(ListConversationMessages)
const sendCustomerTextMessageBound = bind(SendCustomerTextMessage)
const startDirectConversationBound = bind(StartDirectConversation)
const sendDirectTextMessageBound = bind(SendDirectTextMessage)
const createGroupConversationBound = bind(CreateGroupConversation)
const getGroupConversationBound = bind(GetGroupConversation)
const sendGroupTextMessageBound = bind(SendGroupTextMessage)
const listCustomerServiceAssigneesBound = bind(ListCustomerServiceAssignees)
const claimServiceSessionBound = bind(ClaimServiceSession)
const transferServiceSessionBound = bind(TransferServiceSession)
const closeServiceSessionBound = bind(CloseServiceSession)
const reopenServiceSessionBound = bind(ReopenServiceSession)

export type LoadInboxQuery = Partial<LoadInboxInput>

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
  return { ...result, messages: asList(result.messages) }
}

/** 发送成员客户会话文本消息。 */
export function sendCustomerTextMessage(
  conversationID: string,
  input: CustomerTextMessageInput,
) {
  return sendCustomerTextMessageBound(conversationID, input)
}

/** 发起或打开企业成员内部单聊。 */
export function startDirectConversation(input: DirectConversationInput) {
  return startDirectConversationBound(input)
}

/** 发送企业成员内部单聊文本消息。 */
export function sendDirectTextMessage(
  conversationID: string,
  input: DirectTextMessageInput,
) {
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
  const result = await getGroupConversationBound(conversationID)
  return { ...result, participants: asList(result.participants) }
}

/** 发送企业内部群聊文本消息。 */
export function sendGroupTextMessage(
  conversationID: string,
  input: GroupTextMessageInput,
) {
  return sendGroupTextMessageBound(conversationID, input)
}
