/** 消息列表路由。 */
import { useRef } from "react"
import { useSearchParams } from "react-router"

import {
  CustomerInboxView,
  InboxScope,
  type InboxQuery,
} from "@/api"
import { useWorkspace } from "@/contexts/workspace-context"
import { useAttachmentQueue } from "./attachment-queue-context"
import { useMemberChatPollingActive } from "./use-member-chat-polling"
import { InboxPage } from "@/features/inbox/inbox-page"
import { normalizeInboxListQuery } from "./inbox-list-controller"
import { useInboxList } from "./use-inbox-list"
import { useInboxListViewport } from "./use-inbox-list-viewport"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 以规范化查询作为选择历史的保存键。 */
function browseKey(query: InboxQuery) {
  return JSON.stringify(normalizeInboxListQuery(query))
}

/** 加载并显示消息页。 */
export function InboxRoute() {
  const selections = useRef(new Map<string, string>())
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedConversationId = searchParams.get("conversation") ?? ""
  const scope =
    optionalWailsEnum(InboxScope, searchParams.get("scope")) ??
    InboxScope.InboxScopeAll
  const customerView =
    optionalWailsEnum(CustomerInboxView, searchParams.get("view")) ??
    CustomerInboxView.CustomerInboxViewQueue
  const assigneeIdentityId =
    scope === InboxScope.InboxScopeCustomer &&
    customerView === CustomerInboxView.CustomerInboxViewCoworkers
      ? (searchParams.get("assignee") ?? "")
      : ""
  const query = { scope, customerView, assigneeIdentityId }
  const viewport = useInboxListViewport()
  const { identity, beginUnreadSnapshot, applyUnreadSnapshot } = useWorkspace()
  const { queue } = useAttachmentQueue()
  const active = useMemberChatPollingActive()
  const list = useInboxList(query, viewport, {
    identity, active, selectedConversationId,
    unread: (count) => applyUnreadSnapshot(count, beginUnreadSnapshot()),
    unavailable: (id) => queue?.forgetConversation(id),
  })

  /** 更新收件箱范围和客户视图查询参数。 */
  function updateQuery(changes: {
    scope?: InboxScope
    customerView?: CustomerInboxView
    assigneeIdentityId?: string
    conversationId?: string
    replace?: boolean
  }) {
    const nextScope = changes.scope ?? scope
    const nextView = changes.customerView ?? customerView
    const nextAssignee = changes.assigneeIdentityId ?? assigneeIdentityId
    const currentKey = browseKey(query)
    const nextKey = browseKey({ scope: nextScope, customerView: nextView, assigneeIdentityId: nextAssignee })
    // 切换筛选时保存当前选择，恢复目标筛选的上次选择，无记录则保持未选中。
    selections.current.set(currentKey, selectedConversationId)
    const nextSelection = changes.conversationId ?? (nextKey === currentKey ? selectedConversationId : selections.current.get(nextKey) ?? "")
    setSearchParams((current) => {
      const next = new URLSearchParams(current)
      next.delete("target")
      if (nextScope === InboxScope.InboxScopeAll) next.delete("scope")
      else next.set("scope", nextScope)
      if (nextScope === InboxScope.InboxScopeCustomer) {
        if (nextView === CustomerInboxView.CustomerInboxViewQueue)
          next.delete("view")
        else next.set("view", nextView)
        if (
          nextView === CustomerInboxView.CustomerInboxViewCoworkers &&
          nextAssignee
        )
          next.set("assignee", nextAssignee)
        else next.delete("assignee")
      } else {
        next.delete("view")
        next.delete("assignee")
      }
      if (nextSelection) next.set("conversation", nextSelection)
      else next.delete("conversation")
      return next
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
      scope={scope}
      customerView={customerView}
      assigneeIdentityId={assigneeIdentityId}
      selectedConversationId={selectedConversationId}
      targetIdentityId={searchParams.get("target") ?? ""}
      onSelectedConversationChange={selectConversation}
      onQueryChange={updateQuery}
    />
  )
}
