/** 移动端会话列表外壳：会话行、置顶排序、长按菜单、加载与空状态，以及列表区左右滑动。 */
import { useEffect, useRef, useState, type ReactNode } from "react"
import { GripVerticalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type CustomerInboxConversationData,
  type AgentInboxConversationData,
  type DirectInboxConversationData,
  type InboxConversation,
  type GroupInboxConversationData,
} from "@/api"
import {
  MobilePageHeader,
  MobilePageState,
} from "@/apps/mobile/mobile-page"
import {
  mobileConversationPath,
  useMobileNavigation,
} from "@/apps/mobile/mobile-navigation"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  ConversationListMenu,
  useConversationListActions,
} from "@/features/inbox/conversation-list-menu"
import { ConversationRowContent } from "@/features/inbox/conversation-row-content"
import { InboxListPanel } from "@/features/inbox/inbox-list-panel"
import {
  PinnedConversationList,
  pinMoveCommand,
  pinSortableStyle,
  usePinSortable,
  type PinSortable,
} from "@/features/inbox/pinned-sort"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import { useMinuteTick } from "@/features/inbox/use-conversation-time"
import type {
  InboxList,
  InboxListViewport,
  PartitionedInboxList,
} from "@/features/inbox/use-inbox-list"
import { useMemberChatPollingActive } from "@/features/inbox/use-member-chat-polling"
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

type MobileConversationRowProps = {
  conversation: MobileInboxConversation
  name: string
  actions: ReturnType<typeof useConversationListActions>
  pinOrderVersion: string
  pinMoves?: NonNullable<Parameters<typeof ConversationListMenu>[0]["pinMoves"]>
  showAssignee: boolean
  showAudience: boolean
  sorting: boolean
  onMenuChange: (open: boolean) => void
  onOpen: (conversation: MobileInboxConversation) => void
  sortable?: PinSortable
}

/** 渲染会话摘要和未读角标，点击进入会话详情，长按打开阅读状态与置顶菜单；排序模式下置顶行右侧显示拖动手柄。 */
function MobileConversationRow({
  conversation,
  name,
  actions,
  pinOrderVersion,
  pinMoves,
  showAssignee,
  showAudience,
  sorting,
  onMenuChange,
  onOpen,
  sortable,
}: MobileConversationRowProps) {
  const { t } = useTranslation("inbox")
  return (
    <li
      ref={sortable?.setNodeRef}
      style={pinSortableStyle(sortable)}
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
            <ConversationRowContent
              conversation={conversation}
              name={name}
              density="touch"
              showAssignee={showAssignee}
              showAudience={showAudience}
            />
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
  const sortable = usePinSortable(
    props.conversation.id,
    !props.sorting || props.actions.saving,
  )
  return <MobileConversationRow {...props} sortable={sortable} />
}

/** 读取移动端列表所需的身份、轮询与浏览位置选项。 */
export function useMobileListOptions() {
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const { identity } = useMobileWorkspace()
  const { inboxWindows } = useMobileNavigation()
  return { identity, active: pollingActive, history: inboxWindows }
}

/** 渲染带标题栏的会话列表，排序模式下标题栏只保留完成按钮；onSwipe 返回是否切换了范围。 */
export function MobileConversationListPage({
  title,
  actions,
  toolbar,
  list,
  viewport,
  showAssignee,
  showAudience,
  emptyTitle,
  emptyDescription,
  onSwipe,
}: {
  title: string
  actions: ReactNode
  toolbar: ReactNode
  list: InboxList | PartitionedInboxList
  viewport: InboxListViewport
  showAssignee: boolean
  showAudience: boolean
  emptyTitle: string
  emptyDescription: string
  onSwipe?: (direction: 1 | -1) => boolean
}) {
  const { t } = useTranslation(["mobile", "inbox", "common"])
  const conversationName = useConversationName()
  const navigate = useNavigate()
  const actionsState = useConversationListActions(list.settlePin)
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
      conversationName(conversation),
    ]),
  )
  const initial = list.revision === 0
  const row = (conversation: MobileInboxConversation) => ({
    conversation,
    name: names.get(conversation.id) ?? "",
    actions: actionsState,
    pinOrderVersion: list.pinOrderVersion,
    showAssignee,
    showAudience,
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
        title={title}
        actions={sorting ? (
          <Button
            variant="ghost"
            className="-mr-2 min-h-11"
            onClick={() => setSorting(false)}
          >
            {t("inbox:pinSortDone")}
          </Button>
        ) : actions}
      />
      {toolbar}
      <div
        className="flex min-h-0 flex-1 flex-col"
        onTouchStart={(event) => {
          // 菜单打开期间的触摸只服务于菜单本身，从排序手柄开始的触摸只用于拖动。
          const touch = event.touches[0]
          const onHandle =
            event.target instanceof Element &&
            event.target.closest("[data-pin-sort-handle]") !== null
          swipeStart.current =
            onSwipe && touch && !onHandle && !viewport.interaction.current.menu
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
          if (!onSwipe || !start || !touch || viewport.interaction.current.menu) return
          const moveX = touch.clientX - start.x
          const moveY = touch.clientY - start.y
          // 横向位移达到阈值且是纵向的两倍以上时判定为切换范围的滑动。
          if (Math.abs(moveX) < 64 || Math.abs(moveX) < Math.abs(moveY) * 2)
            return
          // 抬手后的点击不应落到滑动经过的会话行上。
          if (onSwipe(moveX < 0 ? 1 : -1)) event.preventDefault()
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
              title={t("listLoadError")}
              onRetry={() => void list.retry()}
            />
          ) : null}
          {!initial && conversations.length === 0 && !list.hasBefore && !list.hasAfter ? (
            <MobilePageState title={emptyTitle} description={emptyDescription} />
          ) : null}
          {conversations.length > 0 ? (
            <ul>
              <PinnedConversationList
                conversations={conversations}
                pinnedIds={list.pinnedIds}
                names={names}
                pinOrderVersion={list.pinOrderVersion}
                actions={actionsState}
                onDraggingChange={viewport.setDragging}
                renderPinned={(conversation, order, index) => (
                  <SortableMobileConversationRow
                    key={conversation.id}
                    {...row(conversation)}
                    pinMoves={{
                      up: pinMoveCommand(order, conversation.id, index - 1, list.pinOrderVersion),
                      down: pinMoveCommand(order, conversation.id, index + 1, list.pinOrderVersion),
                      sort: () => {
                        exitSortingOnMenuClose.current = false
                        setSorting(true)
                      },
                    }}
                  />
                )}
                renderRow={(conversation) => (
                  <MobileConversationRow key={conversation.id} {...row(conversation)} />
                )}
              />
            </ul>
          ) : null}
        </InboxListPanel>
      </div>
    </section>
  )
}
