/** 移动端统一会话摘要列表、阅读状态菜单和内部聊天入口。 */
import { useRef } from "react"
import { BellOffIcon, PlusIcon, SearchIcon } from "lucide-react"
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
  ServiceSessionStatus,
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
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { LoadingIndicator } from "@/components/loading-indicator"
import { sessionStatusLabel } from "@/features/inbox/session-status-label"
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
import { useInboxList } from "@/features/inbox/use-inbox-list"
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

/** 渲染会话摘要和未读角标，点击进入会话详情，长按打开阅读状态菜单。 */
function MobileConversationRow({
  conversation,
  actions,
  onMenuChange,
  onOpen,
}: {
  conversation: MobileInboxConversation
  actions: ReturnType<typeof useConversationListActions>
  onMenuChange: (open: boolean) => void
  onOpen: (conversation: MobileInboxConversation) => void
}) {
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
  const name = customerConversation
    ? (customerConversation.customer.contactName ?? t("anonymousVisitor"))
    : (
        directConversation?.direct.peerName ??
        (agent
          ? `${agent.agentName} · ${agent.title}`
          : groupConversation?.group.title)
      )?.trim() || t("unknownSender")
  const summary =
    customerConversation?.customer ??
    directConversation?.direct ??
    agent ??
    groupConversation?.group
  const agentRunLabel = agentRunStatusLabel(agent?.agentRunStatus ?? null, t)
  // 返回客服处理状态的移动端颜色。
  const customerSessionStatusClass =
    customerConversation?.customer.serviceSessionStatus ===
    ServiceSessionStatus.ServiceSessionStatusOpen
      ? "bg-primary/10 text-primary"
      : "bg-muted text-muted-foreground"

  if (!summary) return null
  const preview =
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
  const formattedTime = formatTime(summary.lastMessageAt)
  const internalConversation =
    directConversation ??
    groupConversation ??
    (isAgentInboxConversation(conversation) ? conversation : null)

  const content = (
    <>
      <span className="relative shrink-0">
        <ConversationAvatar conversation={conversation} />
        <ConversationUnreadBadge conversation={conversation} />
      </span>
      <div className="min-w-0 flex-1 overflow-hidden">
        <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <p className="min-w-0 flex-1 truncate text-[15px] font-medium">
              {name}
            </p>
            {agentRunLabel ? (
              <span className="shrink-0 text-[10px] text-muted-foreground">
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
        {customerConversation ? (
          <div className="mt-1.5 flex min-w-0 items-center gap-2">
            <span className="truncate text-xs text-muted-foreground">
              {customerConversation.customer.channelName}
            </span>
            <span
              className={cn(
                "ml-auto shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium",
                customerSessionStatusClass,
              )}
            >
              {sessionStatusLabel(
                customerConversation.customer.serviceSessionStatus,
                t,
              )}
            </span>
          </div>
        ) : null}
      </div>
    </>
  )

  return (
    <li data-inbox-id={conversation.id} className="border-b last:border-b-0">
      <ConversationListMenu
        conversation={conversation}
        actions={actions}
        itemClassName="min-h-11"
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
    </li>
  )
}

/** 加载当前范围的真实会话摘要并恢复列表浏览位置。 */
export function MobileInboxPage() {
  const navigation = useMobileInboxQuery()
  return <MobileInboxList key={JSON.stringify(navigation.query)} {...navigation} />
}

/** 每个移动筛选独立挂载窗口，离开时保存原邻域。 */
function MobileInboxList({ query, changeQuery }: ReturnType<typeof useMobileInboxQuery>) {
  const { t } = useTranslation(["mobile", "inbox", "common"])
  const navigate = useNavigate()
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const { identity } = useMobileWorkspace()
  const { inboxWindows } = useMobileNavigation()
  const viewport = useInboxListViewport()
  const list = useInboxList(query, viewport, {
    identity,
    active: pollingActive,
    history: inboxWindows,
  })
  const actions = useConversationListActions()
  const swipeStart = useRef<{ x: number; y: number } | null>(null)
  useMinuteTick()
  const conversations = list.conversations.filter(isMobileInboxConversation)
  const initial = list.revision === 0

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t("inbox.title")}
        actions={
          <>
            <Button
              variant="ghost"
              size="icon-lg"
              className="shrink-0"
              aria-label={t("inbox:searchLabel")}
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
        }
      />
      <MobileInboxScopes
        scope={query.scope}
        attentionUnreadCount={list.attentionUnreadCount}
        onChange={changeQuery}
      />
      <div className="flex h-11 shrink-0 items-center border-b">
        <MobileInboxFilter query={query} onChange={changeQuery} onOpenChange={viewport.setMenu} />
      </div>
      <div
        className="flex min-h-0 flex-1 flex-col"
        onTouchStart={(event) => {
          // 菜单打开期间的触摸只服务于菜单本身。
          const touch = event.touches[0]
          swipeStart.current =
            touch && !viewport.interaction.current.menu
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
              {conversations.map((conversation) => (
                <MobileConversationRow
                  key={conversation.id}
                  conversation={conversation}
                  actions={actions}
                  onMenuChange={viewport.setMenu}
                  onOpen={(conversation) =>
                    navigate(mobileConversationPath(conversation), {
                      state: { conversation, mobileBack: true },
                    })
                  }
                />
              ))}
            </ul>
          ) : null}
        </InboxListPanel>
      </div>
    </section>
  )
}
