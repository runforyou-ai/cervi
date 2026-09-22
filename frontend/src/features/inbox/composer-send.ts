/** 按会话类型调用对应的成员文本消息发送接口。 */
import {
  ConversationType,
  MessageVisibility,
  sendAgentTextMessage,
  sendCustomerCopilotTextMessage,
  sendCustomerTextMessage,
  sendDirectTextMessage,
  sendGroupTextMessage,
  type ConversationMessageData,
  type DirectTextMessageInput,
} from "@/api"
import type { MentionTarget } from "@/features/inbox/outgoing-message-store"

/** 按会话类型发送一条成员文本消息；单聊类草稿首发走调用方提供的入口。 */
export function sendComposerTextMessage({
  conversationType,
  conversationID,
  clientMessageID,
  body,
  replyToMessageID,
  visibility,
  mentions,
  mentionAll,
  sendIndividualMessage,
}: {
  conversationType: ConversationType
  conversationID: string
  clientMessageID: string
  body: string
  replyToMessageID: string
  visibility: MessageVisibility
  mentions: MentionTarget[]
  mentionAll: boolean
  sendIndividualMessage?: (
    input: DirectTextMessageInput,
  ) => Promise<ConversationMessageData>
}): Promise<ConversationMessageData> {
  const messageInput = { clientMessageId: clientMessageID, body }
  switch (conversationType) {
    case ConversationType.ConversationTypeAgent:
    case ConversationType.ConversationTypeCopilot:
    case ConversationType.ConversationTypeDirect: {
      const directInput = {
        ...messageInput,
        replyToMessageId: replyToMessageID,
      }
      // 草稿首发走调用方入口，已有会话按类型发送。
      if (sendIndividualMessage) return sendIndividualMessage(directInput)
      if (conversationType === ConversationType.ConversationTypeAgent)
        return sendAgentTextMessage(conversationID, directInput)
      if (conversationType === ConversationType.ConversationTypeCopilot)
        return sendCustomerCopilotTextMessage(conversationID, directInput)
      return sendDirectTextMessage(conversationID, directInput)
    }
    case ConversationType.ConversationTypeGroup:
      return sendGroupTextMessage(conversationID, {
        ...messageInput,
        replyToMessageId: replyToMessageID,
        mentionSubjectIds: mentions.flatMap((mention) =>
          mention.chatSubjectID ? [mention.chatSubjectID] : [],
        ),
        mentionAll,
      })
    case ConversationType.ConversationTypeCustomer:
      return sendCustomerTextMessage(conversationID, {
        ...messageInput,
        replyToMessageId: replyToMessageID,
        visibility,
        mentionIdentityIds: visibility === MessageVisibility.MessageVisibilityInternalOnly
          ? mentions.map((mention) => mention.identityID)
          : [],
      })
    default:
      throw new Error("不支持的会话类型")
  }
}
