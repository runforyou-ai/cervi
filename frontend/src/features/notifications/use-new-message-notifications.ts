/** 把成员事件流确认的新消息与客服处理周期提醒接入各端的本地通知投递。 */
import { useEffect, useEffectEvent } from "react"
import { useTranslation } from "react-i18next"

import {
  MessageType,
  MessageVisibility,
  getInboxConversation,
  isNotFoundApiError,
  InboxScope,
  loadInbox,
  readConversationAttention,
  realtimeClient,
  type ConversationAttentionMessage,
  type Identity,
  type InboxConversationData,
} from "@/api"
import type {
  RealtimeServerFrame,
  ServiceAttentionReason,
} from "@/api/realtime/protocol"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import { NewMessageWatcher } from "./new-message-watcher"
import { notifyNewMessage } from "./new-message-notifications"

/** 各提醒原因对应的通知正文。 */
const attentionBodyKeys = {
  assigned: "notificationServiceAssigned",
  response_overdue: "notificationServiceResponseOverdue",
  queue_waiting: "notificationServiceQueueWaiting",
  returned: "notificationServiceReturned",
} as const satisfies Record<ServiceAttentionReason, string>

/** 登录身份就绪后观察新消息与客服处理周期提醒并投递本地通知，投递成功时回调调用方。 */
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
    async (conversation: InboxConversationData, message: ConversationAttentionMessage) => {
      if (!organizationId || !userId) {
        return
      }
      // 附件消息展示文件名，运行失败使用固定文案，其余消息使用正文摘要。
      const preview = message.attachmentName
        ? t("notificationAttachment", { name: message.attachmentName })
        : message.type === MessageType.MessageTypeAgentError
          ? t("agentRunFailed")
          : message.preview
      const sender = message.senderName?.trim() || t("unknownSender")
      const delivered = await notifyNewMessage({
        id: message.id,
        title: conversationName(conversation),
        // 内部备注标明来源，与客户消息区分。
        body:
          message.visibility === MessageVisibility.MessageVisibilityInternal
            ? t("notificationInternalNoteBody", { sender, preview })
            : conversation.group
              ? t("notificationGroupBody", { sender, preview })
              : preview,
        scope: { organizationId, userId },
      })
      if (delivered) {
        onDelivered()
      }
    },
  )

  const deliverAttention = useEffectEvent(
    async (frame: Extract<RealtimeServerFrame, { type: "service_attention" }>) => {
      if (!organizationId || !userId) {
        return
      }
      let conversation: InboxConversationData
      try {
        conversation = await getInboxConversation(frame.conversationId)
      } catch (error) {
        // 会话已失去阅读资格时结束本次提醒。
        if (isNotFoundApiError(error)) return
        throw error
      }
      const delivered = await notifyNewMessage({
        // 服务端每次提醒对应一次新的分配或等待轮次，通知编号按到达时间区分，不替换上一轮的通知。
        id: `service_attention:${frame.serviceSessionId}:${frame.reason}:${Date.now()}`,
        title: conversationName(conversation),
        body: t(attentionBodyKeys[frame.reason]),
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
    const watcher = new NewMessageWatcher({
      // 基线覆盖本人参与的聊天与待处理的服务会话。
      readConversations: async () => {
        const [chats, pending] = await Promise.all([
          loadInbox({ scope: InboxScope.InboxScopeChat }),
          loadInbox({ scope: InboxScope.InboxScopePending }),
        ])
        return [...chats.conversations, ...pending.conversations]
      },
      readAttention: async (conversationId, afterMessageId) => {
        try {
          return await readConversationAttention(conversationId, { afterMessageId })
        } catch (error) {
          // 失去阅读资格的会话按不可读处理，由观察器清除其基线。
          if (isNotFoundApiError(error)) return null
          throw error
        }
      },
      deliver,
      failed: (error) => {
        console.warn("处理新消息通知失败", error)
      },
    })
    const unsubscribe = realtimeClient.subscribe((event) => {
      if (event.type !== "frame") {
        return
      }
      // 服务端对每次分配与每轮等待只发一次提醒，客户端逐条投递。
      if (event.frame.type === "service_attention") {
        void deliverAttention(event.frame).catch((error: unknown) => {
          console.warn("处理客服提醒通知失败", error)
        })
        return
      }
      watcher.receive(event.frame)
    })
    // 订阅时事件流可能已经建立，此时不会再收到问候事件，直接取一次基线。
    if (realtimeClient.state === "ready") {
      watcher.start()
    }
    return () => {
      unsubscribe()
      watcher.dispose()
    }
    // deliver 是稳定的 Effect Event，依赖只保留 identityId，观察器在登录期间只创建一次并持续持有基线与去重集合。
  }, [identityId])
}
