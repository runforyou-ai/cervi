/** 读取时间线窗口内持久消息的对客投递状态和引用状态。 */
import { useEffect, useMemo, useRef } from "react"
import { replaceEqualDeep } from "@tanstack/react-query"

import {
  listConversationMessageReferences,
  listCustomerMessageDeliveries,
  type ConversationMessageReferenceState,
  type CustomerMessageDelivery,
} from "@/api"
import { useRealtimeSyncActive } from "@/contexts/realtime-sync-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

import type { TimelineMessage } from "./timeline-messages"

type TimelineMessageStateIndex = {
  deliveries: Partial<Record<string, CustomerMessageDelivery>>
  references: Partial<Record<string, ConversationMessageReferenceState>>
}

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
  // 按消息编号沿用内容未变的状态对象，窗口变化和重读后未变化的消息行保持 memo。
  const previousIndex = useRef<TimelineMessageStateIndex>({ deliveries: {}, references: {} })
  const index = useMemo(() => replaceEqualDeep(previousIndex.current, {
    deliveries: Object.fromEntries(deliveries.data?.deliveries.map((delivery) => [delivery.messageId, delivery]) ?? []),
    references: Object.fromEntries(references.data?.states.map((state) => [state.messageId, state]) ?? []),
  }), [deliveries.data, references.data])
  useEffect(() => {
    previousIndex.current = index
  }, [index])
  return {
    deliveries,
    deliveriesByMessage: index.deliveries,
    referencesByMessage: index.references,
  }
}
