/** 会话列表项的摘要数据与末条消息摘要文案，桌面端与移动端列表共用。 */
import type { TFunction } from "i18next"

import {
  ConversationStatus,
  MessageType,
  MessageVisibility,
  isAgentInboxConversation,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type InboxConversationData,
} from "@/api"
import { messagePreview } from "@/lib/message-preview"

/** 返回会话列表项的摘要数据（末条消息、时间、未读等）；列表不支持的会话类型返回 null。 */
export function inboxConversationSummary(conversation: InboxConversationData) {
  if (isCustomerInboxConversation(conversation)) return conversation.customer
  if (isAgentInboxConversation(conversation)) return conversation.agent
  if (isDirectInboxConversation(conversation)) return conversation.direct
  if (isGroupInboxConversation(conversation)) return conversation.group
  return null
}

/** 会话列表项的摘要文案：群解散、运行结果与内部备注各有固定文案，其余取末条消息预览。 */
export function conversationPreview(
  conversation: InboxConversationData,
  t: TFunction<"inbox">,
) {
  const summary = inboxConversationSummary(conversation)
  const groupDissolved =
    isGroupInboxConversation(conversation) &&
    conversation.group.status ===
      ConversationStatus.ConversationStatusArchived
  const previewBody = groupDissolved
    ? t("groupDissolved")
    : conversation.lastMessageType === MessageType.MessageTypeAgentCancelled
      ? t("agentReplyStopped")
      : conversation.lastMessageType === MessageType.MessageTypeAgentError
        ? t("agentRunFailed")
        : messagePreview(
            summary?.preview ?? "",
            summary?.previewSenderIdentityType,
          ).trim() ||
          (isGroupInboxConversation(conversation) && summary?.lastMessageAt
            ? t("groupSystemUpdated")
            : t("messagesEmpty"))
  // 客户会话的末条消息是内部备注时，摘要标明来源。
  return isCustomerInboxConversation(conversation) &&
    conversation.customer.previewVisibility ===
      MessageVisibility.MessageVisibilityInternalOnly
    ? t("previewInternalNote", { preview: previewBody })
    : previewBody
}
