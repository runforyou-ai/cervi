/** 成员收件箱与会话消息调用归一化。 */
import {
  ListConversationMessages,
  LoadInbox,
  SendCustomerTextMessage,
  SendDirectTextMessage,
  StartDirectConversation,
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
  Inbox,
  InboxConversation,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { ConversationType } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
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
}

export type DirectInboxConversationData = InboxConversation & {
  type: ConversationType.ConversationTypeDirect
  customer: null
  direct: DirectInboxConversation
}

const loadInboxBound = bind(LoadInbox)
const listConversationMessagesBound = bind(ListConversationMessages)
const sendCustomerTextMessageBound = bind(SendCustomerTextMessage)
const startDirectConversationBound = bind(StartDirectConversation)
const sendDirectTextMessageBound = bind(SendDirectTextMessage)

/** 判断统一收件箱项是否为结构完整的客户会话。 */
export function isCustomerInboxConversation(
  conversation: InboxConversation,
): conversation is CustomerInboxConversationData {
  return (
    conversation.type === ConversationType.ConversationTypeCustomer &&
    conversation.customer !== null &&
    conversation.direct === null
  )
}

/** 判断统一收件箱项是否为结构完整的内部单聊。 */
export function isDirectInboxConversation(
  conversation: InboxConversation,
): conversation is DirectInboxConversationData {
  return (
    conversation.type === ConversationType.ConversationTypeDirect &&
    conversation.customer === null &&
    conversation.direct !== null
  )
}

/** 读取成员统一收件箱会话列表。 */
export async function loadInbox(): Promise<InboxData> {
  const inbox = await loadInboxBound()
  return {
    ...inbox,
    conversations: asList(inbox.conversations),
  }
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
