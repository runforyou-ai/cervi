/** 读取时间线窗口内持久消息的对客投递状态和引用状态。 */
import {
  listConversationMessageReferences,
  listCustomerMessageDeliveries,
} from "@/api"
import { useRealtimeSyncActive } from "@/contexts/realtime-sync-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

import type { TimelineMessage } from "./timeline-messages"

/** 按窗口内持久消息编号查询投递与引用状态，并按消息编号建立索引。 */
export function useTimelineMessageStates({
  conversationID,
  messages,
  enabled,
  pollingActive,
}: {
  conversationID: string
  messages: TimelineMessage[]
  enabled: boolean
  pollingActive: boolean
}) {
  const realtime = useRealtimeSyncActive()
  // 使用窗口内持久消息编号查询投递，历史窗口也能刷新原有消息的状态。
  const messageIDs = messages
    .flatMap((message) => message.persistedMessageID ? [message.persistedMessageID] : [])
    .sort()
    .join(",")
  const deliveries = useResource(
    resourceKeys.customerDeliveries(conversationID, messageIDs),
    () => listCustomerMessageDeliveries(conversationID, messageIDs),
    {
      enabled: enabled && Boolean(messageIDs),
      keepPreviousData: true,
      refetchInterval: pollingActive && !realtime ? 2000 : false,
    },
  )
  const references = useResource(
    resourceKeys.conversationMessageReferences(conversationID, messageIDs),
    () => listConversationMessageReferences(conversationID, messageIDs),
    {
      enabled: enabled && Boolean(messageIDs),
      keepPreviousData: true,
      refetchInterval: pollingActive && !realtime ? 2000 : false,
    },
  )
  return {
    deliveries,
    deliveriesByMessage: new Map(deliveries.data?.deliveries.map((delivery) => [delivery.messageId, delivery])),
    referencesByMessage: new Map(references.data?.states.map((state) => [state.messageId, state])),
  }
}
