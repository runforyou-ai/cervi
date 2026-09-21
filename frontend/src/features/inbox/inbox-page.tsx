/** 消息页中栏（会话列表）和会话主区。 */
import { useEffect, useRef, useState } from "react"
import { MessagesSquareIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  ConversationType,
  CustomerInboxView,
  CustomerQueueFilter,
  InboxScope,
  OrganizationIdentityType,
  ServiceSessionStatus,
  isApiError,
  isCustomerInboxConversation,
  listCustomerServiceAssignees,
  listInboxChannels,
  listServiceQueueTeams,
  openConversationWindow,
  type AgentInboxConversationData,
  type DirectInboxConversationData,
  type GroupInboxConversationData,
  type InboxConversation,
  type MemberOption,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageSplit } from "@/components/page-split"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useGlobalSearch } from "@/contexts/global-search-context"
import { useWorkspace } from "@/contexts/workspace-context"
import { ConversationDetail } from "@/features/inbox/conversation-detail"
import { ConversationMain } from "@/features/inbox/conversation-main"
import type { ConversationLocateTarget } from "@/features/inbox/conversation-timeline"
import { ConversationTargetPickerDialog } from "@/features/inbox/conversation-target-picker-dialog"
import { CreateGroupConversationDialog } from "@/features/inbox/create-group-conversation-dialog"
import { InboxConversationList } from "@/features/inbox/inbox-conversation-list"
import { InboxConversationTarget } from "@/features/inbox/inbox-conversation-target"
import { InboxCustomerQueueFilter } from "@/features/inbox/inbox-customer-queue-filter"
import { InboxFilter } from "@/features/inbox/inbox-filter"
import { InboxListPanel } from "@/features/inbox/inbox-list-panel"
import { InboxPaneTop } from "@/features/inbox/inbox-pane-top"
import type { ChatDraft } from "@/features/inbox/inbox-selection"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import {
  readConversationSummary,
  useConversationSummary,
} from "@/features/inbox/use-conversation-summary"
import type { PartitionedInboxList } from "@/features/inbox/use-inbox-list"
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
  queueFilter,
  queueTeamId,
  assigneeIdentityId,
  channelId,
  serviceStatus,
  kinds,
  selectedConversationId,
  targetIdentityId,
  locateMessage,
  onSelectedConversationChange,
  onQueryChange,
}: {
  list: PartitionedInboxList
  listViewport: ReturnType<typeof useInboxListViewport>
  scope: InboxScope
  customerView: CustomerInboxView
  queueFilter: CustomerQueueFilter
  queueTeamId: string
  assigneeIdentityId: string
  channelId: string
  serviceStatus: ServiceSessionStatus
  kinds: ConversationType[]
  selectedConversationId: string
  targetIdentityId: string
  locateMessage: ConversationLocateTarget | null
  onSelectedConversationChange: (
    conversationId: string,
    replace?: boolean,
  ) => void
  onQueryChange: (changes: {
    scope?: InboxScope
    customerView?: CustomerInboxView
    queueFilter?: CustomerQueueFilter
    queueTeamId?: string
    assigneeIdentityId?: string
    channelId?: string
    serviceStatus?: ServiceSessionStatus
    kinds?: ConversationType[]
    conversationId?: string
    replace?: boolean
  }) => void
}) {
  const { conversations, attentionUnreadCount, customerMentionedUnreadCount } = list
  // 根据当前窗口、前后分页资格和首次读取状态判断会话列表是否有效。
  const hasConversations = conversations.length > 0 || list.hasBefore || list.hasAfter || list.revision === 0
  const { t } = useTranslation(["inbox", "common"])
  const { identity } = useWorkspace()
  const isNarrowViewport = useIsNarrowViewport()
  const invalidate = useResourceInvalidator()
  const readResource = useResourceReader()
  const globalSearch = useGlobalSearch()
  const [chatDraft, setChatDraft] = useState<ChatDraft | null>(null)
  const [isNarrowDetailOpen, setIsNarrowDetailOpen] = useState(false)
  const [agentDialogOpen, setAgentDialogOpen] = useState(false)
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const navigationGeneration = useRef(0)
  const summary = useConversationSummary(targetIdentityId ? "" : selectedConversationId)
  const selectedConversation = summary.data ?? undefined
  const recordRecentConversation = useRecentConversations(identity.user.identityId).record
  const openedConversationId = selectedConversation?.id
  const [messageTarget, setMessageTarget] = useState<ConversationLocateTarget | null>(locateMessage)
  useEffect(() => {
    if (openedConversationId) recordRecentConversation(openedConversationId)
  }, [openedConversationId, recordRecentConversation])
  useEffect(() => {
    // 搜索结果带来的定位目标随地址进入，切换会话时由选择动作清除。
    setMessageTarget(locateMessage)
  }, [locateMessage])

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
  const { data: queueTeams = [] } = useResource(
    resourceKeys.serviceQueueTeams(),
    () => listServiceQueueTeams(),
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

  /** 从会话头进入当前会话的搜索范围。 */
  function searchConversation(conversationID: string) {
    globalSearch?.open(conversationID)
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

  /** 主动退群后清空选择并关闭窄屏详情，不打开其他会话。 */
  function showConversationAfterGroupLeft() {
    setChatDraft(null)
    setIsNarrowDetailOpen(false)
    onSelectedConversationChange("", true)
  }

  /** 本人关闭客户会话后取消选中并清空主区；接管、发消息、转交或重开后，客户列表的归属筛选跟随该会话的去向。 */
  function followCustomerConversation(conversation: InboxConversation) {
    if (!isCustomerInboxConversation(conversation) || conversation.id !== selectedConversationId) return
    if (conversation.customer.serviceSessionStatus === ServiceSessionStatus.ServiceSessionStatusClosed) {
      // 操作前仍在处理中即本次操作是关闭；已关闭会话里的其他操作保持选中。
      const closing = selectedConversation && isCustomerInboxConversation(selectedConversation) &&
        selectedConversation.customer.serviceSessionStatus === ServiceSessionStatus.ServiceSessionStatusOpen
      if (closing) {
        setIsNarrowDetailOpen(false)
        onQueryChange({ conversationId: "" })
      }
      return
    }
    if (scope !== InboxScope.InboxScopeCustomer) return
    const assigneeId = conversation.customer.assignee?.identityId ?? ""
    // 在 @我的 视图中留言或由他人负责时保持当前视图，本人接手后跟随到我负责的。
    if (customerView === CustomerInboxView.CustomerInboxViewMentioned && assigneeId !== identity.user.identityId) return
    const nextView = !assigneeId
      ? CustomerInboxView.CustomerInboxViewQueue
      : assigneeId === identity.user.identityId
        ? CustomerInboxView.CustomerInboxViewMine
        : CustomerInboxView.CustomerInboxViewCoworkers
    // 同事视图已按其他客服筛选时改为新的负责人，未筛选时保持查看全部同事。
    const nextAssignee = nextView === CustomerInboxView.CustomerInboxViewCoworkers && assigneeIdentityId ? assigneeId : ""
    // 跟随到「待分配」时一律按全部队列。
    const nextQueueFilter = nextView === CustomerInboxView.CustomerInboxViewQueue
      ? CustomerQueueFilter.CustomerQueueFilterAll
      : CustomerQueueFilter.$zero
    if (nextView === customerView && nextAssignee === assigneeIdentityId && nextQueueFilter === queueFilter && serviceStatus === ServiceSessionStatus.ServiceSessionStatusOpen) return
    onQueryChange({ customerView: nextView, assigneeIdentityId: nextAssignee, queueFilter: nextQueueFilter, queueTeamId: "", serviceStatus: ServiceSessionStatus.ServiceSessionStatusOpen, conversationId: conversation.id })
  }

  /** 按当前聊天草稿或选中会话渲染主区内容。 */
  function renderConversation(narrowViewport: boolean) {
    if (activeChatDraft) {
      return (
        <ConversationMain
          selection={activeChatDraft}
          onChatStarted={showStartedConversation}
          onSearchConversation={searchConversation}
          locateMessage={messageTarget}
          narrowViewport={narrowViewport}
        />
      )
    }
    if (!selectedConversationId) return null
    return (
      <ConversationDetail
        conversationId={selectedConversationId}
        summary={summary}
        onGroupLeft={showConversationAfterGroupLeft}
        onLocalChange={followCustomerConversation}
        onSearchConversation={searchConversation}
        locateMessage={messageTarget}
        narrowViewport={narrowViewport}
      />
    )
  }

  const pane = (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <InboxPaneTop
        scope={scope}
        attentionUnreadCount={attentionUnreadCount}
        customerMentionedUnreadCount={customerMentionedUnreadCount}
        onScopeChange={(nextScope) => {
          setChatDraft(null)
          onQueryChange({ scope: nextScope })
        }}
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
          queueFilter={queueFilter}
          queueTeamId={queueTeamId}
          queueTeams={queueTeams}
          assigneeIdentityId={assigneeIdentityId}
          assignees={customerServiceAssignees}
          currentIdentityId={identity.user.identityId}
          onChange={(change) =>
            onQueryChange({
              customerView: change.customerView,
              assigneeIdentityId: change.assigneeIdentityId ?? "",
              queueFilter: change.queueFilter ?? CustomerQueueFilter.$zero,
              queueTeamId: change.queueTeamId ?? "",
            })
          }
        />
      ) : null}
      <InboxListPanel list={list} viewport={listViewport} detailError={Boolean(summary.error)} retryDetail={() => void summary.refresh()}>
        <InboxConversationList
          conversations={conversations}
          showQueueTeam={
            scope === InboxScope.InboxScopeCustomer &&
            customerView === CustomerInboxView.CustomerInboxViewQueue &&
            queueFilter === CustomerQueueFilter.CustomerQueueFilterAll
          }
          pinnedIds={list.pinnedIds}
          pinOrderVersion={list.pinOrderVersion}
          onMenuChange={listViewport.setMenu}
          onDraggingChange={listViewport.setDragging}
          onPinSettled={list.settlePin}
          selectedId={selectedConversation?.id}
          onSelect={selectConversation}
          onOpenInWindow={
            resolveAppPlatform() === "desktop"
              ? (conversation, name) => void openConversationInWindow(conversation, name)
              : undefined
          }
        />
      </InboxListPanel>
    </div>
  )

  return (
    <>
      <PageSplit
        paneWidth="inbox"
        paneOnNarrow="fill"
        className="bg-background"
        pane={pane}
      >
        {isNarrowViewport ? null : targetIdentityId ? (
          <LoadingIndicator className="flex-1 justify-center">
            {t("chatTargetLoading")}
          </LoadingIndicator>
        ) : activeChatDraft || selectedConversationId ? (
          <section className="flex min-h-0 flex-1 flex-col">
            {renderConversation(false)}
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

      {isNarrowViewport && (activeChatDraft || selectedConversationId) ? (
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
            {renderConversation(true)}
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
