/** 消息页中栏（范围纵栏 + 会话列表）和会话主区。 */
import { useEffect, useRef, useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { MessagesSquareIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  ConversationType,
  CustomerInboxView,
  InboxScope,
  InboxSearchPersonKind,
  OrganizationIdentityType,
  ServiceSessionStatus,
  findDirectConversation,
  isApiError,
  listCustomerServiceAssignees,
  listInboxChannels,
  openConversationWindow,
  sessionPath,
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
import type { ConversationLocateTarget } from "@/features/inbox/conversation-timeline"
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
import { InboxSearchPanel } from "@/features/inbox/inbox-search-panel"
import type {
  ChatDraft,
  ConversationSelection,
} from "@/features/inbox/inbox-selection"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import {
  readConversationSummary,
  useConversationSummary,
} from "@/features/inbox/use-conversation-summary"
import { useInboxSearch, type InboxSearchItem } from "@/features/inbox/use-inbox-search"
import type { InboxList } from "@/features/inbox/use-inbox-list"
import { useRecentConversations } from "@/features/inbox/use-recent-conversations"
import type { useInboxListViewport } from "@/features/inbox/use-inbox-list-viewport"
import { resourceKeys } from "@/hooks/resource-keys"
import { useIsNarrowViewport } from "@/hooks/use-narrow-viewport"
import {
  useResource,
  useResourceInvalidator,
  useResourceReader,
} from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { resolveAppPlatform } from "@/platform/app-platform"

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
  const paneRef = useRef<HTMLDivElement>(null)
  const railToggledRef = useRef(false)
  const [chatDraft, setChatDraft] = useState<ChatDraft | null>(null)
  const [isNarrowDetailOpen, setIsNarrowDetailOpen] = useState(false)
  const [agentDialogOpen, setAgentDialogOpen] = useState(false)
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const navigationGeneration = useRef(0)
  const summary = useConversationSummary(targetIdentityId ? "" : selectedConversationId)
  const selectedConversation = summary.data ?? undefined
  const recentConversations = useRecentConversations(identity.user.identityId)
  const recordRecentConversation = recentConversations.record
  const openedConversationId = selectedConversation?.id
  const locateNonce = useRef(0)
  const [messageTarget, setMessageTarget] = useState<({ conversationId: string } & ConversationLocateTarget) | null>(null)
  const search = useInboxSearch({
    query: { scope, customerView, assigneeIdentityId, channelId, serviceStatus, kinds },
    recentConversationIds: recentConversations.ids,
    onOpen: (item) => void openSearchItem(item),
  })
  useEffect(() => {
    if (openedConversationId) recordRecentConversation(openedConversationId)
  }, [openedConversationId, recordRecentConversation])

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
    setMessageTarget(null)
    onSelectedConversationChange(conversationId)

    if (isNarrowViewport) {
      setIsNarrowDetailOpen(true)
    }
  }

  /** 打开搜索结果：消息结果保留搜索并定位原消息，其余结果退出搜索后打开会话或聊天草稿。 */
  async function openSearchItem(item: InboxSearchItem) {
    if (item.kind === "message") {
      // 先切换会话再写入定位目标，切换会话会清除旧目标。
      if (item.message.conversation.id !== selectedConversationId) selectConversation(item.message.conversation.id)
      else if (isNarrowViewport) setIsNarrowDetailOpen(true)
      locateNonce.current++
      setMessageTarget({ conversationId: item.message.conversation.id, messageId: item.message.id, nonce: locateNonce.current })
      return
    }
    if (item.kind === "conversation") {
      search.exit()
      selectConversation(item.conversation.id)
      return
    }
    const { person } = item
    if (person.kind === InboxSearchPersonKind.InboxSearchPersonContact) {
      if (!person.conversationId) return
      search.exit()
      selectConversation(person.conversationId)
      return
    }
    search.exit()
    const member: MemberOption = {
      id: person.id,
      type: person.identityType ?? OrganizationIdentityType.OrganizationIdentityTypeUser,
      displayName: person.displayName,
      avatarUrl: person.avatarUrl,
    }
    if (member.type === OrganizationIdentityType.OrganizationIdentityTypeAgent) {
      showChatDraft(member)
      return
    }
    // 真人成员复用已有单聊，读取期间切换了导航时放弃本次打开。
    const generation = navigationGeneration.current
    try {
      const existing = await readResource(resourceKeys.directConversation(member.id), () => findDirectConversation(member.id))
      if (generation === navigationGeneration.current) showChatDraft(member, existing)
    } catch (error) {
      if (isApiError(error) && sessionPath(error.state)) return
      console.warn("打开搜索成员聊天失败", { identityId: member.id, error })
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("directLookupError"))
    }
  }

  /** 从会话头进入当前会话的搜索范围。 */
  function searchConversation(conversationID: string) {
    if (isNarrowViewport) setIsNarrowDetailOpen(false)
    search.enter(conversationID)
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

  /** 桌面端在独立窗口打开会话，窗口标题取会话名称。 */
  async function openConversationInWindow(conversation: InboxConversation, name: string) {
    try {
      await openConversationWindow({ conversationId: conversation.id, title: name })
    } catch (error) {
      console.warn("打开会话独立窗口失败", { conversationId: conversation.id, error })
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("conversationWindowOpenError"))
    }
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

  // 收起和展开由两个位置的按钮分别承担，切换后把焦点交给新出现的那个。
  useEffect(() => {
    if (!railToggledRef.current) return
    railToggledRef.current = false
    paneRef.current
      ?.querySelector<HTMLButtonElement>("[data-slot=rail-toggle]")
      ?.focus()
  }, [railCollapsed])

  /** 切换范围栏并标记本次由用户操作触发。 */
  function toggleRail(collapsed: boolean) {
    railToggledRef.current = true
    setRailCollapsed(collapsed)
  }

  const pane = (
    <div ref={paneRef} className="flex min-h-0 flex-1">
      {railCollapsed ? null : (
        <InboxScopeRail
          scope={scope}
          attentionUnreadCount={attentionUnreadCount}
          onScopeChange={(nextScope) => {
            setChatDraft(null)
            onQueryChange({ scope: nextScope })
          }}
          onCollapse={() => toggleRail(true)}
        />
      )}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <InboxPaneTop
          railCollapsed={railCollapsed}
          onRailExpand={() => toggleRail(false)}
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
          search={search}
        />
        {search.active ? (
          <InboxSearchPanel search={search} scope={scope} />
        ) : null}
        {!search.active && scope === InboxScope.InboxScopeCustomer ? (
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
        {search.active ? null : (
          <InboxListPanel list={list} viewport={listViewport} detailError={Boolean(summary.error)} retryDetail={() => void summary.refresh()}>
            <InboxConversationList
              conversations={conversations}
              onMenuChange={listViewport.setMenu}
              selectedId={selectedConversation?.id}
              onSelect={selectConversation}
              onOpenInWindow={
                resolveAppPlatform() === "desktop"
                  ? (conversation, name) => void openConversationInWindow(conversation, name)
                  : undefined
              }
            />
          </InboxListPanel>
        )}
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
              onSearchConversation={searchConversation}
              locateMessage={messageTarget}
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
          <SheetContent
            className="data-[side=right]:w-full p-0 sm:max-w-lg"
            onCloseAutoFocus={(event) => {
              // 搜索模式下关闭详情时，把焦点交给中栏搜索框。
              if (!search.active) return
              event.preventDefault()
              search.inputRef.current?.focus()
            }}
          >
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
              onSearchConversation={searchConversation}
              locateMessage={messageTarget}
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
