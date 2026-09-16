/** 各端独立读取会话摘要，并在确认失权后清理共享资源。 */
import { useEffect, useRef } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { getInboxConversation, isNotFoundApiError } from "@/api"
import { useRealtimeSyncActive } from "@/contexts/realtime-sync-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { useAttachmentQueue } from "./attachment-queue-context"
import { useOutgoingMessageStore } from "./outgoing-message-context"
import { clearConversationResources } from "./conversation-resources"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "./use-member-chat-polling"

/** 将服务端确认的不可用结果保存为查询事实，网络错误保留既有详情。 */
export async function readConversationSummary(conversationID: string, signal?: AbortSignal) {
  try {
    return await getInboxConversation(conversationID, signal)
  } catch (error) {
    if (isNotFoundApiError(error)) return null
    throw error
  }
}

/** 当前会话按变更通知或前台轮询发现资料变化，恢复前台后立即重读。 */
export function useConversationSummary(conversationID: string, requireWindowFocus = true) {
  const client = useQueryClient()
  const { queue } = useAttachmentQueue()
  const outgoingStore = useOutgoingMessageStore()
  const active = useMemberChatPollingActive({ requireWindowFocus })
  const previousActive = useRef(active)
  const realtime = useRealtimeSyncActive()
  // 接入实时同步的外壳由会话通知与失权通知失效摘要，其余外壳在前台轮询；不可用时继续读取，重新获得阅读资格后恢复详情。
  const resource = useResource(
    resourceKeys.conversationSummary(conversationID),
    (signal) => readConversationSummary(conversationID, signal),
    {
      enabled: Boolean(conversationID),
      staleTime: 0,
      refetchInterval: active && !realtime ? memberChatPollingInterval : false,
      refetchOnWindowFocus: false,
    },
  )
  const { data, refresh } = resource
  useEffect(() => {
    if (active && !previousActive.current && conversationID) void refresh()
    previousActive.current = active
  }, [active, conversationID, refresh])
  useEffect(() => {
    if (data !== null || !conversationID) return
    queue?.forgetConversation(conversationID)
    outgoingStore.forgetConversation(conversationID)
    clearConversationResources(client, conversationID)
  }, [client, conversationID, data, queue, outgoingStore])
  return resource
}
