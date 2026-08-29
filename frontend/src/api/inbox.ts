/** 成员收件箱与会话消息调用归一化。 */
import {
  ListConversationMessages,
  LoadInbox,
  SendCustomerTextMessage,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  ConversationMessage,
  ConversationMessageList,
  ConversationMessageListInput,
  CustomerTextMessageInput,
  Inbox,
  InboxConversation,
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

const loadInboxBound = bind(LoadInbox)
const listConversationMessagesBound = bind(ListConversationMessages)
const sendCustomerTextMessageBound = bind(SendCustomerTextMessage)

/** 读取成员收件箱的客户会话列表。 */
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
