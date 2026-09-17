/** 把成员事件流确认的新消息接入 Web 与桌面端的本地通知投递。 */
import { useEffect, useEffectEvent } from "react"
import { useTranslation } from "react-i18next"

import {
  getInboxConversation,
  isNotFoundApiError,
  listConversationMessages,
  listPendingConversationMentions,
  loadInbox,
  realtimeClient,
  type ConversationMessage,
  type Identity,
  type InboxConversation,
} from "@/api"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import { messagePreview } from "@/lib/message-preview"
import { NewMessageWatcher } from "./new-message-watcher"
import { notifyNewMessage } from "./new-message-notifications"

/** 登录身份就绪后观察新消息并投递本地通知，投递成功时上报一次提醒。 */
export function useNewMessageNotifications(
  identity: Identity | null,
  onDelivered: () => void,
) {
  const { t } = useTranslation("inbox")
  const conversationName = useConversationName()
  const organizationId = identity?.organization.id
  const userId = identity?.user.id
  const identityId = identity?.user.identityId

  const deliver = useEffectEvent(
    async (conversation: InboxConversation, message: ConversationMessage) => {
      if (!organizationId || !userId) {
        return
      }
      // 附件消息展示文件名，其余消息按发送者身份取正文摘要。
      const preview = message.attachment
        ? t("notificationAttachment", { name: message.attachment.name })
        : messagePreview(message.body, message.sender?.identityType).trim()
      const sender = message.sender?.displayName?.trim() || t("unknownSender")
      const delivered = await notifyNewMessage({
        id: message.id,
        title: conversationName(conversation),
        body: conversation.group
          ? t("notificationGroupBody", { sender, preview })
          : preview,
        scope: { organizationId, userId },
      })
      if (delivered) {
        onDelivered()
      }
    },
  )

  useEffect(() => {
    if (!identityId) {
      return
    }
    const watcher = new NewMessageWatcher(identityId, {
      readConversations: async () => (await loadInbox()).conversations,
      readConversation: async (conversationId) => {
        try {
          return await getInboxConversation(conversationId)
        } catch (error) {
          // 失去阅读资格时不再保留该会话的基线。
          if (isNotFoundApiError(error)) return null
          throw error
        }
      },
      readMessages: async (conversationId) =>
        (await listConversationMessages(conversationId)).messages,
      readPendingMentions: async (conversationId) =>
        (await listPendingConversationMentions(conversationId)).messageIds,
      deliver,
      failed: (error) => {
        console.warn("处理新消息通知失败", error)
      },
    })
    const unsubscribe = realtimeClient.subscribe((event) => {
      if (event.type === "frame") {
        watcher.receive(event.frame)
      }
    })
    return () => {
      unsubscribe()
      watcher.dispose()
    }
    // deliver 是稳定的 Effect Event，不进入依赖，避免每次渲染重建观察器。
  }, [identityId])
}
