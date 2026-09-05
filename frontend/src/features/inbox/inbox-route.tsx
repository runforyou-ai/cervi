/** 消息列表路由。 */
import { useEffect, useRef } from "react"
import { RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import {
  CustomerInboxView,
  InboxScope,
  loadInbox,
  type InboxData,
} from "@/api"
import { Button } from "@/components/ui/button"
import { LoadingIndicator } from "@/components/loading-indicator"
import { InboxPage } from "@/features/inbox/inbox-page"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { useWorkspace } from "@/contexts/workspace-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 加载并显示消息页。 */
export function InboxRoute() {
  const { t } = useTranslation("workspace")
  const { applyUnreadSnapshot, beginUnreadSnapshot } = useWorkspace()
  const pollingActive = useMemberChatPollingActive()
  const previousPollingActiveRef = useRef(pollingActive)
  const previousDataRef = useRef<InboxData | null>(null)
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
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.inbox(query),
    () => loadInbox(query),
    {
      refetchInterval: pollingActive ? memberChatPollingInterval : false,
      refetchOnWindowFocus: false,
    },
  )
  const showLoading = loading || (Boolean(error) && refreshing)
  if (data) previousDataRef.current = data
  const visibleData = data ?? previousDataRef.current

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

  if (showLoading && !visibleData) {
    return (
      <LoadingIndicator className="flex-1 justify-center">
        {t("loading")}
      </LoadingIndicator>
    )
  }

  if ((!data && Boolean(error)) || !visibleData) {
    return (
      <div className="flex flex-1 items-center justify-center p-6 text-center">
        <div>
          <p className="text-sm text-muted-foreground">
            {t("inboxLoadError")}
          </p>
          <Button
            className="mt-4"
            variant="outline"
            onClick={() => void refresh()}
          >
            <RefreshCwIcon />
            {t("retry")}
          </Button>
        </div>
      </div>
    )
  }

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
    setSearchParams((current) => {
      const next = new URLSearchParams(current)
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
      if (changes.conversationId !== undefined) {
        if (changes.conversationId) {
          next.set("conversation", changes.conversationId)
        } else {
          next.delete("conversation")
        }
      }
      return next
    }, { replace: changes.replace ?? true })
  }

  /** 将当前会话同步到地址，支持刷新和前进后退恢复。 */
  function selectConversation(conversationId: string, replace = false) {
    setSearchParams((current) => {
      const next = new URLSearchParams(current)
      if (conversationId) next.set("conversation", conversationId)
      else next.delete("conversation")
      return next
    }, { replace })
  }

  return (
    <InboxPage
      conversations={visibleData.conversations}
      attentionUnreadCount={visibleData.attentionUnreadCount}
      listLoading={showLoading}
      listError={Boolean(error)}
      onListRefresh={() => void refresh()}
      scope={scope}
      customerView={customerView}
      assigneeIdentityId={assigneeIdentityId}
      selectedConversationId={selectedConversationId}
      onSelectedConversationChange={selectConversation}
      onQueryChange={updateQuery}
    />
  )
}
