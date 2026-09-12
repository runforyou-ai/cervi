/** 消息列表路由。 */
import { useRef } from "react"
import { useSearchParams } from "react-router"

import {
  ConversationType,
  CustomerInboxView,
  InboxScope,
  ServiceSessionStatus,
  type InboxQuery,
} from "@/api"
import { useWorkspace } from "@/contexts/workspace-context"
import { useAttachmentQueue } from "./attachment-queue-context"
import { useOutgoingMessageStore } from "./outgoing-message-context"
import { useMemberChatPollingActive } from "./use-member-chat-polling"
import { InboxPage } from "@/features/inbox/inbox-page"
import {
  inboxQueryFromSearch,
  normalizeInboxQuery,
  writeInboxQuerySearch,
} from "./inbox-query"
import { useInboxList } from "./use-inbox-list"
import { useInboxListViewport } from "./use-inbox-list-viewport"

/** 以规范化查询作为选择历史的保存键。 */
function browseKey(query: InboxQuery) {
  return JSON.stringify(query)
}

/** 加载并显示消息页。 */
export function InboxRoute() {
  const selections = useRef(new Map<string, string>())
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedConversationId = searchParams.get("conversation") ?? ""
  const query = inboxQueryFromSearch(searchParams)
  const viewport = useInboxListViewport()
  const { identity, beginUnreadSnapshot, applyUnreadSnapshot } = useWorkspace()
  const { queue } = useAttachmentQueue()
  const outgoingStore = useOutgoingMessageStore()
  const active = useMemberChatPollingActive()
  const list = useInboxList(query, viewport, {
    identity, active, selectedConversationId,
    unread: (count) => applyUnreadSnapshot(count, beginUnreadSnapshot()),
    unavailable: (id) => {
      queue?.forgetConversation(id)
      outgoingStore.forgetConversation(id)
    },
  })

  /** 更新收件箱范围和客户视图查询参数。 */
  function updateQuery(changes: {
    scope?: InboxScope
    customerView?: CustomerInboxView
    assigneeIdentityId?: string
    channelId?: string
    serviceStatus?: ServiceSessionStatus
    kinds?: ConversationType[]
    conversationId?: string
    replace?: boolean
  }) {
    const next = normalizeInboxQuery({ ...query, ...changes })
    const currentKey = browseKey(query)
    const nextKey = browseKey(next)
    // 切换筛选时保存当前选择，恢复目标筛选的上次选择，无记录则保持未选中。
    selections.current.set(currentKey, selectedConversationId)
    const nextSelection = changes.conversationId ?? (nextKey === currentKey ? selectedConversationId : selections.current.get(nextKey) ?? "")
    setSearchParams((current) => {
      const params = new URLSearchParams(current)
      params.delete("target")
      writeInboxQuerySearch(params, next)
      if (nextSelection) params.set("conversation", nextSelection)
      else params.delete("conversation")
      return params
    }, { replace: changes.replace ?? true })
  }

  /** 将当前会话同步到地址，支持刷新和前进后退恢复。 */
  function selectConversation(conversationId: string, replace = false) {
    setSearchParams((current) => {
      const next = new URLSearchParams(current)
      next.delete("target")
      if (conversationId) next.set("conversation", conversationId)
      else next.delete("conversation")
      return next
    }, { replace })
  }

  return (
    <InboxPage
      list={list}
      listViewport={viewport}
      scope={query.scope}
      customerView={query.customerView}
      assigneeIdentityId={query.assigneeIdentityId}
      channelId={query.channelId}
      serviceStatus={query.serviceStatus}
      kinds={query.kinds}
      selectedConversationId={selectedConversationId}
      targetIdentityId={searchParams.get("target") ?? ""}
      onSelectedConversationChange={selectConversation}
      onQueryChange={updateQuery}
    />
  )
}
