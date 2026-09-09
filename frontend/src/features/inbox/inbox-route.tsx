/** 消息列表路由。 */
import { useRef } from "react"
import { useSearchParams } from "react-router"

import {
  CustomerInboxView,
  InboxScope,
} from "@/api"
import { InboxPage } from "@/features/inbox/inbox-page"
import { useInboxList } from "./use-inbox-list"
import { useInboxListViewport } from "./use-inbox-list-viewport"
import { optionalWailsEnum } from "@/lib/wails-enum"

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
  const list = useInboxList(query, viewport)
  viewport.positions.current = list.positions

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
    const queryIdentity = `${scope}/${customerView}/${assigneeIdentityId}`
    const nextQueryIdentity = `${nextScope}/${nextScope === InboxScope.InboxScopeCustomer ? nextView : CustomerInboxView.CustomerInboxViewQueue}/${nextScope === InboxScope.InboxScopeCustomer && nextView === CustomerInboxView.CustomerInboxViewCoworkers ? nextAssignee : ""}`
    // 切换筛选时保存当前选择，恢复目标筛选的上次选择，无记录则保持未选中。
    selections.current.set(queryIdentity, selectedConversationId)
    const nextSelection = changes.conversationId ?? (nextQueryIdentity === queryIdentity ? selectedConversationId : selections.current.get(nextQueryIdentity) ?? "")
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
