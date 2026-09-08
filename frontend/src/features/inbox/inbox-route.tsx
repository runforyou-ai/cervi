/** 消息列表路由。 */
import { useEffect, useRef } from "react"
import { useSearchParams } from "react-router"

import {
  CustomerInboxView,
  InboxScope,
  loadInbox,
  type InboxConversation,
} from "@/api"
import { InboxPage } from "@/features/inbox/inbox-page"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { useWorkspace } from "@/contexts/workspace-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { optionalWailsEnum } from "@/lib/wails-enum"

const emptyConversations: InboxConversation[] = []

/** 加载并显示消息页。 */
export function InboxRoute() {
  const { applyUnreadSnapshot, beginUnreadSnapshot } = useWorkspace()
  const pollingActive = useMemberChatPollingActive()
  const previousPollingActiveRef = useRef(pollingActive)
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
  const { data, loading, error, refresh } = useResource(
    resourceKeys.inbox(query),
    () => loadInbox(query),
    {
      refetchInterval: pollingActive ? memberChatPollingInterval : false,
      refetchOnWindowFocus: false,
    },
  )
  const showLoading = loading && !data

  useEffect(() => {
    if (pollingActive && !previousPollingActiveRef.current && data) {
      void refresh()
    }
    previousPollingActiveRef.current = pollingActive
  }, [data, pollingActive, refresh])

  /** 数据就绪或更新后同步未读快照；命中缓存的重新挂载同样生效。 */
  useEffect(() => {
    if (!data) return
    const unreadRevision = beginUnreadSnapshot()
    applyUnreadSnapshot(data.attentionUnreadCount, unreadRevision)
    console.info("消息已加载", {
      conversation_count: data.conversations.length,
    })
  }, [applyUnreadSnapshot, beginUnreadSnapshot, data])

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
      conversations={data?.conversations ?? emptyConversations}
      attentionUnreadCount={data?.attentionUnreadCount ?? 0}
      listLoading={showLoading}
      listError={Boolean(error)}
      onListRefresh={() => void refresh()}
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
