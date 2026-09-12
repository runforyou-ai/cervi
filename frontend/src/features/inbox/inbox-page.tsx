/** 消息页中栏（范围纵栏 + 会话列表）和会话主区。 */
import { useEffect, useRef, useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { MessagesSquareIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  ConversationType,
  CustomerInboxView,
  InboxScope,
  OrganizationIdentityType,
  ServiceSessionStatus,
  listCustomerServiceAssignees,
  listInboxChannels,
  markConversationRead,
  type AgentInboxConversationData,
  type DirectInboxConversationData,
  type GroupInboxConversationData,
  type InboxConversation,
  type MemberOption,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageSplit } from "@/components/page-split"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useWorkspace } from "@/contexts/workspace-context"
import { useAttachmentQueue } from "@/features/inbox/attachment-queue-context"
import { useOutgoingMessageStore } from "@/features/inbox/outgoing-message-context"
import { ConversationMain } from "@/features/inbox/conversation-main"
import { clearConversationResources } from "@/features/inbox/conversation-resources"
import { ConversationTargetPickerDialog } from "@/features/inbox/conversation-target-picker-dialog"
import { CreateGroupConversationDialog } from "@/features/inbox/create-group-conversation-dialog"
import { InboxConversationList } from "@/features/inbox/inbox-conversation-list"
import { InboxConversationTarget } from "@/features/inbox/inbox-conversation-target"
import { InboxCustomerQueueFilter } from "@/features/inbox/inbox-customer-queue-filter"
import { InboxFilter } from "@/features/inbox/inbox-filter"
import { InboxListPanel } from "@/features/inbox/inbox-list-panel"
import { InboxPaneTop } from "@/features/inbox/inbox-pane-top"
import { InboxScopeRail } from "@/features/inbox/inbox-scope-rail"
import type {
  ChatDraft,
  ConversationSelection,
} from "@/features/inbox/inbox-selection"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import {
  readConversationSummary,
  useConversationSummary,
} from "@/features/inbox/use-conversation-summary"
import type { InboxList } from "@/features/inbox/use-inbox-list"
import type { useInboxListViewport } from "@/features/inbox/use-inbox-list-viewport"
import { resourceKeys } from "@/hooks/resource-keys"
import { useIsNarrowViewport } from "@/hooks/use-narrow-viewport"
import {
  useResource,
  useResourceInvalidator,
  useResourceReader,
} from "@/hooks/use-resource"

/** 新建后需要切换到的内部会话。 */
type InternalInboxConversationData =
  | AgentInboxConversationData
  | DirectInboxConversationData
  | GroupInboxConversationData

/** 消息页中栏和当前会话。 */
export function InboxPage({
  list,
  listViewport,
  scope,
  customerView,
  assigneeIdentityId,
  channelId,
  serviceStatus,
  kinds,
  selectedConversationId,
  targetIdentityId,
  onSelectedConversationChange,
  onQueryChange,
}: {
  list: InboxList
  listViewport: ReturnType<typeof useInboxListViewport>
  scope: InboxScope
  customerView: CustomerInboxView
  assigneeIdentityId: string
  channelId: string
  serviceStatus: ServiceSessionStatus
  kinds: ConversationType[]
  selectedConversationId: string
  targetIdentityId: string
  onSelectedConversationChange: (
    conversationId: string,
    replace?: boolean,
  ) => void
  onQueryChange: (changes: {
    scope?: InboxScope
    customerView?: CustomerInboxView
    assigneeIdentityId?: string
    channelId?: string
    serviceStatus?: ServiceSessionStatus
    kinds?: ConversationType[]
    conversationId?: string
    replace?: boolean
  }) => void
}) {
  const { conversations, attentionUnreadCount } = list
  // 根据当前窗口、前后分页资格和首次读取状态判断会话列表是否有效。
  const hasConversations = conversations.length > 0 || list.hasBefore || list.hasAfter || list.revision === 0
  const { t } = useTranslation(["inbox", "common"])
  const { identity } = useWorkspace()
  const isNarrowViewport = useIsNarrowViewport()
  const invalidate = useResourceInvalidator()
  const readResource = useResourceReader()
  const queryClient = useQueryClient()
  const { queue } = useAttachmentQueue()
  const outgoingStore = useOutgoingMessageStore()
  const [railCollapsed, setRailCollapsed] = useState(false)
  const [chatDraft, setChatDraft] = useState<ChatDraft | null>(null)
  const [isNarrowDetailOpen, setIsNarrowDetailOpen] = useState(false)
  const [agentDialogOpen, setAgentDialogOpen] = useState(false)
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const navigationGeneration = useRef(0)
  const summary = useConversationSummary(targetIdentityId ? "" : selectedConversationId)
  const selectedConversation = summary.data ?? undefined
  useEffect(() => {
    navigationGeneration.current++
    return () => { navigationGeneration.current++ }
  }, [scope, customerView, assigneeIdentityId, selectedConversationId, targetIdentityId, chatDraft])
  useEffect(() => {
    if (isNarrowViewport && selectedConversationId) setIsNarrowDetailOpen(true)
  }, [isNarrowViewport, selectedConversationId])
  useEffect(() => {
    listViewport.setCovered(isNarrowViewport && isNarrowDetailOpen)
    return () => listViewport.setCovered(false)
  }, [isNarrowViewport, isNarrowDetailOpen, listViewport])
  const conversationName = useConversationName()
  const { data: customerServiceAssignees = [] } = useResource(
    resourceKeys.customerServiceAssignees(),
    () => listCustomerServiceAssignees(),
    { enabled: scope === InboxScope.InboxScopeCustomer },
  )
  const { data: channels = [] } = useResource(
    resourceKeys.inboxChannels(),
    () => listInboxChannels(),
    { enabled: scope === InboxScope.InboxScopeCustomer },
  )

  const activeChatDraft =
    !targetIdentityId &&
    scope === InboxScope.InboxScopeInternal &&
    !selectedConversationId
      ? chatDraft
      : null
  useEffect(() => {
    if (selectedConversationId || scope !== InboxScope.InboxScopeInternal) {
      setChatDraft(null)
    }
  }, [scope, selectedConversationId])
  /** 选中一个会话。 */
  function selectConversation(conversationId: string) {
    navigationGeneration.current++
    setChatDraft(null)
    onSelectedConversationChange(conversationId)

    if (isNarrowViewport) {
      setIsNarrowDetailOpen(true)
    }
  }

  /** 不打开会话并把列表项推进到当前最后消息。 */
  async function markConversationAsRead(conversation: InboxConversation) {
    if (!conversation.lastMessageId) return
    await markConversationRead(conversation.id, {
      lastReadMessageId: conversation.lastMessageId,
      clearUnreadMark: true,
    })
  }

  /** 新建后先读取权威摘要，再将草稿连续切换为正式会话。 */
  async function showStartedConversation(conversation: InternalInboxConversationData) {
    const generation = navigationGeneration.current
    try {
      await readResource(resourceKeys.conversationSummary(conversation.id), (signal) => readConversationSummary(conversation.id, signal))
    } catch (error) {
      console.warn("读取新建会话摘要失败", { conversationId: conversation.id, error })
    }
    void invalidate(resourceKeys.inbox())
    if (generation !== navigationGeneration.current) return
    setChatDraft(null)
    onQueryChange({ scope: InboxScope.InboxScopeInternal, conversationId: conversation.id, replace: Boolean(targetIdentityId) })
    setIsNarrowDetailOpen(isNarrowViewport)
  }

  /** 打开已有真人单聊，或在主区开始真人和 AI 聊天草稿。 */
  function showChatDraft(
    member: MemberOption,
    existing: DirectInboxConversationData | null = null,
  ) {
    if (existing) {
      showStartedConversation(existing)
      return
    }
    setChatDraft(
      member.type === OrganizationIdentityType.OrganizationIdentityTypeAgent
        ? { kind: "agent-draft", member, conversationId: crypto.randomUUID() }
        : { kind: "direct-draft", member },
    )
    onQueryChange({
      scope: InboxScope.InboxScopeInternal,
      conversationId: "",
    })
    setIsNarrowDetailOpen(isNarrowViewport)
  }

  /** 消息或客服处理保存后刷新列表与详情，保持当前筛选和选择。 */
  function refreshConversationAfterMessage(conversationID: string) {
    void invalidate(resourceKeys.inbox())
    void invalidate(resourceKeys.conversationSummary(conversationID))
  }

  /** 主动退群后清空选择并关闭窄屏详情，不打开其他会话。 */
  function showConversationAfterGroupLeft(conversationID: string) {
    queue?.forgetConversation(conversationID)
    outgoingStore.forgetConversation(conversationID)
    clearConversationResources(queryClient, conversationID)
    void queryClient.resetQueries({ queryKey: resourceKeys.conversationSummary(conversationID) })
    setChatDraft(null)
    setIsNarrowDetailOpen(false)
    onSelectedConversationChange("", true)
  }

  const selection: ConversationSelection | null = activeChatDraft
    ? activeChatDraft
    : selectedConversation
      ? { kind: "conversation", conversation: selectedConversation }
      : null

  const detailState = selectedConversationId && !selection ? (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
      {summary.loading ? <LoadingIndicator>{t("messagesLoading")}</LoadingIndicator> : (
        <>
          <p>{t(summary.data === null ? "conversationUnavailable" : "conversationLoadError")}</p>
          {summary.data !== null ? <Button variant="outline" size="sm" onClick={() => void summary.refresh()}>{t("common:actions.retry")}</Button> : null}
        </>
      )}
    </div>
  ) : null

  const pane = (
    <div className="flex min-h-0 flex-1">
      {railCollapsed ? null : (
        <InboxScopeRail
          scope={scope}
          attentionUnreadCount={attentionUnreadCount}
          onScopeChange={(nextScope) => {
            setChatDraft(null)
            onQueryChange({ scope: nextScope })
          }}
        />
      )}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <InboxPaneTop
          railCollapsed={railCollapsed}
          onRailToggle={() => setRailCollapsed((collapsed) => !collapsed)}
          onCreateGroup={() => setGroupDialogOpen(true)}
          onCreateAgent={() => setAgentDialogOpen(true)}
          filter={
            <InboxFilter
              scope={scope}
              value={{ channelId, serviceStatus, kinds }}
              channels={channels}
              onChange={onQueryChange}
            />
          }
        />
        {scope === InboxScope.InboxScopeCustomer ? (
          <InboxCustomerQueueFilter
            view={customerView}
            assigneeIdentityId={assigneeIdentityId}
            assignees={customerServiceAssignees}
            currentIdentityId={identity.user.identityId}
            onChange={(nextView, nextAssigneeIdentityId = "") =>
              onQueryChange({
                customerView: nextView,
                assigneeIdentityId: nextAssigneeIdentityId,
              })
            }
          />
        ) : null}
        <InboxListPanel list={list} viewport={listViewport} detailError={Boolean(summary.error)} retryDetail={() => void summary.refresh()}>
          <InboxConversationList
            conversations={conversations}
            onMenuChange={listViewport.setMenu}
            selectedId={selectedConversation?.id}
            onSelect={selectConversation}
            onMarkRead={markConversationAsRead}
          />
        </InboxListPanel>
      </div>
    </div>
  )

  return (
    <>
      <PageSplit
        paneWidth={railCollapsed ? "inboxCollapsed" : "inbox"}
        paneOnNarrow="fill"
        className="bg-background"
        paneClassName="transition-[width]"
        pane={pane}
      >
        {isNarrowViewport ? null : targetIdentityId ? (
          <LoadingIndicator className="flex-1 justify-center">
            {t("chatTargetLoading")}
          </LoadingIndicator>
        ) : detailState ? detailState : selection ? (
          <section className="min-h-0 flex-1">
            <ConversationMain
              selection={selection}
              onSessionChanged={refreshConversationAfterMessage}
              onConversationChanged={refreshConversationAfterMessage}
              onGroupLeft={showConversationAfterGroupLeft}
              onChatStarted={showStartedConversation}
            />
          </section>
        ) : (
          <div className="cervi-inbox-empty-main flex min-h-0 flex-1 items-center justify-center p-6">
            <div
              data-slot="empty-state-content"
              className="max-w-sm text-center"
            >
              <div className="mx-auto mb-4 flex size-11 items-center justify-center rounded-xl border bg-background shadow-sm">
                <MessagesSquareIcon className="size-5 text-muted-foreground" />
              </div>
              <h2 className="text-base font-semibold tracking-tight">
                {t(hasConversations ? "selectConversationTitle" : "emptyTitle")}
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                {t(hasConversations ? "selectConversationDescription" : "emptyDescription")}
              </p>
            </div>
          </div>
        )}
      </PageSplit>

      {isNarrowViewport && (selection || detailState) ? (
        <Sheet
          open={isNarrowDetailOpen}
          onOpenChange={(open) => {
            setIsNarrowDetailOpen(open)
            if (!open) setChatDraft(null)
          }}
        >
          <SheetContent className="data-[side=right]:w-full p-0 sm:max-w-lg">
            <SheetHeader className="sr-only">
              <SheetTitle>
                {t("conversationTitle", {
                  name:
                    activeChatDraft?.member.displayName ??
                    (selectedConversation
                      ? conversationName(selectedConversation)
                      : ""),
                })}
              </SheetTitle>
              <SheetDescription>{t("detailDescription")}</SheetDescription>
            </SheetHeader>
            {selection ? (
            <ConversationMain
              selection={selection}
              onSessionChanged={refreshConversationAfterMessage}
              onConversationChanged={refreshConversationAfterMessage}
              onGroupLeft={showConversationAfterGroupLeft}
              onChatStarted={showStartedConversation}
              narrowViewport
            />
            ) : detailState}
          </SheetContent>
        </Sheet>
      ) : null}

      {targetIdentityId ? (
        <InboxConversationTarget
          key={targetIdentityId}
          identityId={targetIdentityId}
          currentIdentityId={identity.user.identityId}
          onSelected={showChatDraft}
          onFailed={() => {
            // 打开失败时结束旧草稿并回到内部会话列表。
            setChatDraft(null)
            onQueryChange({ conversationId: "" })
          }}
        />
      ) : null}
      <ConversationTargetPickerDialog
        open={agentDialogOpen}
        onOpenChange={setAgentDialogOpen}
        onSelected={showChatDraft}
      />
      <CreateGroupConversationDialog
        open={groupDialogOpen}
        currentIdentityID={identity.user.identityId}
        onOpenChange={setGroupDialogOpen}
        onCreated={showStartedConversation}
      />
    </>
  )
}
