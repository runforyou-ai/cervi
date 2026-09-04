/** 构建和识别未持久化的单聊草稿。 */
import {
  ConversationType,
  type DirectInboxConversationData,
  type MemberOption,
} from "@/api"

const directConversationDraftPrefix = "draft:"

/** 为所选成员构建仅存在于当前页面的单聊草稿。 */
export function createDirectConversationDraft(
  member: MemberOption,
): DirectInboxConversationData {
  return {
    id: `${directConversationDraftPrefix}${member.id}`,
    type: ConversationType.ConversationTypeDirect,
    unreadCount: 0,
    mentionedUnreadCount: 0,
    muted: false,
    lastMessageId: null,
    lastReadMessageId: null,
    customer: null,
    direct: {
      peerIdentityId: member.id,
      peerType: member.type,
      peerName: member.displayName,
      preview: null,
      lastMessageAt: null,
      agentRunStatus: null,
    },
    group: null,
  }
}

/** 判断会话编号是否属于本地单聊草稿。 */
export function isDirectConversationDraftID(conversationID: string) {
  return conversationID.startsWith(directConversationDraftPrefix)
}
