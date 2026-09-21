/** 移动端统一会话摘要列表、阅读状态与置顶菜单、置顶排序和内部聊天入口。 */
import { useEffect, useRef, useState } from "react"
import type { TFunction } from "i18next"
import { useSortable } from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { BellOffIcon, GripVerticalIcon, PlusIcon, SearchIcon } from "lucide-react"
import { messagePreview } from "@/lib/message-preview"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  ConversationStatus,
  MessageType,
  MessageVisibility,
  type CustomerInboxConversationData,
  type AgentInboxConversationData,
  type DirectInboxConversationData,
  type InboxConversation,
  type GroupInboxConversationData,
} from "@/api"
import {
  inboxScopes,
  MobileInboxFilter,
  MobileInboxScopes,
  useMobileInboxQuery,
} from "@/apps/mobile/mobile-inbox-navigation"
import {
  MobilePageHeader,
  MobilePageState,
} from "@/apps/mobile/mobile-page"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import {
  ConversationListMenu,
  useConversationListActions,
} from "@/features/inbox/conversation-list-menu"
import { ConversationUnreadBadge } from "@/features/inbox/conversation-unread-badge"
import { PinnedSortArea, pinMoveCommand } from "@/features/inbox/pinned-sort"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { LoadingIndicator } from "@/components/loading-indicator"
import { useConversationTime, useMinuteTick } from "@/features/inbox/use-conversation-time"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import {
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import {
  mobileConversationPath,
  mobileSearchPath,
  useMobileNavigation,
} from "@/apps/mobile/mobile-navigation"
import { InboxListPanel } from "@/features/inbox/inbox-list-panel"
import { usePartitionedInboxList } from "@/features/inbox/use-inbox-list"
import { useInboxListViewport } from "@/features/inbox/use-inbox-list-viewport"
import { cn } from "@/lib/utils"

type MobileInboxConversation =
  | CustomerInboxConversationData
  | AgentInboxConversationData
  | DirectInboxConversationData
  | GroupInboxConversationData

/** 识别移动端支持的会话摘要。 */
function isMobileInboxConversation(
  conversation: InboxConversation,
): conversation is MobileInboxConversation {
  return (
    isCustomerInboxConversation(conversation) ||
    isAgentInboxConversation(conversation) ||
    isDirectInboxConversation(conversation) ||
    isGroupInboxConversation(conversation)
  )
}

/** 返回移动端会话行展示的名称。 */
function mobileConversationName(conversation: MobileInboxConversation, t: TFunction<"inbox">) {
  if (isCustomerInboxConversation(conversation))
    return conversation.customer.contactName ?? t("anonymousVisitor")
  const name = isDirectInboxConversation(conversation)
    ? conversation.direct.peerName
    : isAgentInboxConversation(conversation)
      ? `${conversation.agent.agentName} · ${conversation.agent.title}`
      : conversation.group.title
  return name?.trim() || t("unknownSender")
}

type MobileConversationRowProps = {
  conversation: MobileInboxConversation
  name: string
  actions: ReturnType<typeof useConversationListActions>
  pinOrderVersion: string
  pinMoves?: NonNullable<Parameters<typeof ConversationListMenu>[0]["pinMoves"]>
  sorting: boolean
  onMenuChange: (open: boolean) => void
  onOpen: (conversation: MobileInboxConversation) => void
  sortable?: ReturnType<typeof useSortable>
}

/** 渲染会话摘要和未读角标，点击进入会话详情，长按打开阅读状态与置顶菜单；排序模式下置顶行右侧显示拖动手柄。 */
function MobileConversationRow({
  conversation,
  name,
  actions,
  pinOrderVersion,
  pinMoves,
  sorting,
  onMenuChange,
  onOpen,
  sortable,
}: MobileConversationRowProps) {
  const { t } = useTranslation("inbox")
  const formatTime = useConversationTime()
  const customerConversation = isCustomerInboxConversation(conversation)
    ? conversation
    : null
  const directConversation = isDirectInboxConversation(conversation)
    ? conversation
    : null
  const agent = isAgentInboxConversation(conversation)
    ? conversation.agent
    : null
  const groupConversation = isGroupInboxConversation(conversation)
    ? conversation
    : null
  const summary =
    customerConversation?.customer ??
    directConversation?.direct ??
    agent ??
    groupConversation?.group
  const agentRunLabel = agentRunStatusLabel(agent?.agentRunStatus ?? null, t)

  if (!summary) return null
  const previewBody =
    groupConversation?.group.status ===
    ConversationStatus.ConversationStatusArchived
      ? t("groupDissolved")
      : conversation.lastMessageType === MessageType.MessageTypeAgentCancelled
        ? t("agentReplyStopped")
        : conversation.lastMessageType === MessageType.MessageTypeAgentError
          ? t("agentRunFailed")
          : messagePreview(
              summary.preview ?? "",
              summary.previewSenderIdentityType,
            ) ||
            (groupConversation && conversation.lastMessageId
              ? t("groupSystemUpdated")
              : t("messagesEmpty"))
  // 客户会话的末条消息是内部备注时，摘要标明来源。
  const preview =
    customerConversation?.customer.previewVisibility ===
    MessageVisibility.MessageVisibilityInternalOnly
      ? t("previewInternalNote", { preview: previewBody })
      : previewBody
  const formattedTime = formatTime(summary.lastMessageAt)
  const internalConversation =
    directConversation ??
    groupConversation ??
    (isAgentInboxConversation(conversation) ? conversation : null)

  const content = (
    <>
      <span className="relative shrink-0">
        <ConversationAvatar conversation={conversation} className="size-10" />
        <ConversationUnreadBadge conversation={conversation} />
      </span>
      <div className="min-w-0 flex-1 overflow-hidden">
        <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <p className="min-w-0 flex-1 truncate text-[15px] font-medium">
              {name}
            </p>
            {agentRunLabel ? (
              <span className="shrink-0 text-xs text-muted-foreground">
                {agentRunLabel}
              </span>
            ) : null}
          </div>
          {formattedTime ? (
            <time
              dateTime={summary.lastMessageAt ?? undefined}
              className="shrink-0 text-xs text-muted-foreground"
            >
              {formattedTime}
            </time>
          ) : null}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-2">
          <p className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
            {preview}
          </p>
          {internalConversation?.muted ? (
            <BellOffIcon
              className="size-3.5 shrink-0 text-muted-foreground"
              aria-label={t("conversationMuted")}
            />
          ) : null}
        </div>
      </div>
    </>
  )


  return (
    <li
      ref={sortable?.setNodeRef}
      style={
        sortable
          ? {
              transform: CSS.Translate.toString(sortable.transform),
              transition: sortable.transition,
            }
          : undefined
      }
      data-inbox-id={conversation.id}
      data-pinned={conversation.pinned || undefined}
      className={cn(
        "flex min-w-0 items-center border-b last:border-b-0",
        conversation.pinned && "bg-muted",
        sortable?.isDragging && "relative z-10 shadow-md",
      )}
    >
      <div className="min-w-0 flex-1">
        <ConversationListMenu
          conversation={conversation}
          actions={actions}
          itemClassName="min-h-11"
          pinOrderVersion={pinOrderVersion}
          pinMoves={pinMoves}
          onOpenChange={onMenuChange}
        >
          <button
            type="button"
            className="flex w-full min-w-0 gap-3 px-4 py-3 text-left outline-none transition-colors select-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
            aria-label={name}
            onClick={() => onOpen(conversation)}
          >
            {content}
          </button>
        </ConversationListMenu>
      </div>
      {sorting && sortable ? (
        <button
          type="button"
          ref={sortable.setActivatorNodeRef}
          data-pin-sort-handle
          {...sortable.attributes}
          {...sortable.listeners}
          className="flex size-11 shrink-0 touch-none items-center justify-center text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
          aria-label={t("pinSortHandle", { name })}
        >
          <GripVerticalIcon className="size-5" />
        </button>
      ) : null}
    </li>
  )
}

/** 置顶区内的移动端会话行，只在排序模式且未保存时可拖动。 */
function SortableMobileConversationRow(props: MobileConversationRowProps) {
  const { t } = useTranslation("inbox")
  const sortable = useSortable({
    id: props.conversation.id,
    disabled: !props.sorting || props.actions.saving,
    attributes: { roleDescription: t("pinSortRole") },
  })
  return <MobileConversationRow {...props} sortable={sortable} />
}

/** 加载当前范围的真实会话摘要并恢复列表浏览位置。 */
export function MobileInboxPage() {
  const navigation = useMobileInboxQuery()
  return <MobileInboxList key={JSON.stringify(navigation.query)} {...navigation} />
}

/** 每个移动筛选独立挂载窗口，离开时保存原邻域。 */
function MobileInboxList({ query, changeQuery }: ReturnType<typeof useMobileInboxQuery>) {
  const { t } = useTranslation(["mobile", "inbox", "common"])
  const { t: inboxT } = useTranslation("inbox")
  const navigate = useNavigate()
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const { identity } = useMobileWorkspace()
  const { inboxWindows } = useMobileNavigation()
  const viewport = useInboxListViewport()
  const list = usePartitionedInboxList(query, viewport, {
    identity,
    active: pollingActive,
    history: inboxWindows,
  })
  const actions = useConversationListActions(list.settlePin)
  const [sorting, setSorting] = useState(false)
  const exitSortingOnMenuClose = useRef(false)
  const swipeStart = useRef<{ x: number; y: number } | null>(null)
  useMinuteTick()
  useEffect(() => {
    if (!sorting) return
    // 排序期间系统返回先退出排序模式。
    const exitSorting = (event: Event) => {
      if (event.defaultPrevented) return
      event.preventDefault()
      setSorting(false)
    }
    window.addEventListener("cervi:back", exitSorting)
    return () => window.removeEventListener("cervi:back", exitSorting)
  }, [sorting])
  const conversations = list.conversations.filter(isMobileInboxConversation)
  const names = new Map(
    conversations.map((conversation) => [
      conversation.id,
      mobileConversationName(conversation, inboxT),
    ]),
  )
  const rows = new Map(conversations.map((conversation) => [conversation.id, conversation]))
  const initial = list.revision === 0
  const row = (conversation: MobileInboxConversation) => ({
    conversation,
    name: names.get(conversation.id) ?? "",
    actions,
    pinOrderVersion: list.pinOrderVersion,
    sorting,
    // 排序中打开的菜单在关闭后结束排序，菜单打开期间手柄保持占位。
    onMenuChange: (open: boolean) => {
      if (open) exitSortingOnMenuClose.current = sorting
      else if (exitSortingOnMenuClose.current) setSorting(false)
      viewport.setMenu(open)
    },
    onOpen: (conversation: MobileInboxConversation) => {
      setSorting(false)
      navigate(mobileConversationPath(conversation), {
        state: { conversation, mobileBack: true },
      })
    },
  })

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t("inbox.title")}
        actions={sorting ? (
          <Button
            variant="ghost"
            className="-mr-2 min-h-11"
            onClick={() => setSorting(false)}
          >
            {t("inbox:pinSortDone")}
          </Button>
        ) : (
          <>
            <Button
              variant="ghost"
              size="icon-lg"
              className="shrink-0"
              aria-label={t("common:actions.search")}
              onClick={() =>
                navigate(mobileSearchPath(), { state: { mobileBack: true } })
              }
            >
              <SearchIcon />
            </Button>
            <DropdownMenu onOpenChange={viewport.setMenu}>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-lg"
                  className="shrink-0"
                  aria-label={t("inbox.add")}
                >
                  <PlusIcon />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  className="min-h-11"
                  onSelect={() => navigate("/inbox/agent/new", {
                    state: { mobileBack: true },
                  })}
                >
                  {t("inbox:newAgentConversation")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="min-h-11"
                  onSelect={() => navigate("/inbox/group/new", {
                    state: { mobileBack: true },
                  })}
                >
                  {t("group.create")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )}
      />
      <MobileInboxScopes
        scope={query.scope}
        attentionUnreadCount={list.attentionUnreadCount}
        customerMentionedUnreadCount={list.customerMentionedUnreadCount}
        onChange={changeQuery}
      />
      <div className="flex h-11 shrink-0 items-center border-b">
        <MobileInboxFilter query={query} onChange={changeQuery} onOpenChange={viewport.setMenu} />
      </div>
      <div
        className="flex min-h-0 flex-1 flex-col"
        onTouchStart={(event) => {
          // 菜单打开期间的触摸只服务于菜单本身，从排序手柄开始的触摸只用于拖动。
          const touch = event.touches[0]
          const onHandle =
            event.target instanceof Element &&
            event.target.closest("[data-pin-sort-handle]") !== null
          swipeStart.current =
            touch && !onHandle && !viewport.interaction.current.menu
              ? { x: touch.clientX, y: touch.clientY }
              : null
        }}
        onTouchCancel={() => {
          swipeStart.current = null
        }}
        onTouchEnd={(event) => {
          const start = swipeStart.current
          const touch = event.changedTouches[0]
          swipeStart.current = null
          if (!start || !touch || viewport.interaction.current.menu) return
          const moveX = touch.clientX - start.x
          const moveY = touch.clientY - start.y
          // 横向位移达到阈值且是纵向的两倍以上时判定为切换范围的滑动。
          if (Math.abs(moveX) < 64 || Math.abs(moveX) < Math.abs(moveY) * 2)
            return
          const current = inboxScopes.findIndex(
            (item) => item.value === query.scope,
          )
          const next = inboxScopes[current + (moveX < 0 ? 1 : -1)]
          if (!next) return
          // 抬手后的点击不应落到滑动经过的会话行上。
          event.preventDefault()
          changeQuery({ scope: next.value })
        }}
      >
        <InboxListPanel list={list} viewport={viewport} mobile>
          {initial && !list.error ? (
            <LoadingIndicator className="min-h-64 flex-1 justify-center">
              {t("common:status.loading")}
            </LoadingIndicator>
          ) : null}
          {initial && list.error ? (
            <MobilePageState
              title={t("inbox.loadError")}
              onRetry={() => void list.retry()}
            />
          ) : null}
          {!initial && conversations.length === 0 && !list.hasBefore && !list.hasAfter ? (
            <MobilePageState
              title={t("inbox.emptyTitle")}
              description={t("inbox.emptyDescription")}
            />
          ) : null}
          {conversations.length > 0 ? (
            <ul>
              <PinnedSortArea
                conversations={conversations}
                pinnedIds={list.pinnedIds}
                names={names}
                pinOrderVersion={list.pinOrderVersion}
                actions={actions}
                onDraggingChange={viewport.setDragging}
              >
                {(order) => order.flatMap((id, index) => rows.has(id) ? [
                  <SortableMobileConversationRow
                    key={id}
                    {...row(rows.get(id)!)}
                    pinMoves={{
                      up: pinMoveCommand(order, id, index - 1, list.pinOrderVersion),
                      down: pinMoveCommand(order, id, index + 1, list.pinOrderVersion),
                      sort: () => {
                        exitSortingOnMenuClose.current = false
                        setSorting(true)
                      },
                    }}
                  />,
                ] : [])}
              </PinnedSortArea>
              {conversations.flatMap((conversation) => list.pinnedIds.includes(conversation.id) ? [] : [
                <MobileConversationRow key={conversation.id} {...row(conversation)} />,
              ])}
            </ul>
          ) : null}
        </InboxListPanel>
      </div>
    </section>
  )
}
