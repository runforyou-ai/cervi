/** 消息页中栏（范围纵栏 + 会话列表）和会话主区。 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import type { TFunction } from "i18next"
import {
  CheckIcon,
  ChevronDownIcon,
  HeadsetIcon,
  MessagesSquareIcon,
  PanelLeftIcon,
  PlusIcon,
  UsersRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  ChannelType,
  ConversationStatus,
  ConversationType,
  CustomerInboxView,
  InboxScope,
  OrganizationIdentityType,
  ServiceSessionStatus,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  getGroupConversation,
  listCustomerServiceAssignees,
  markConversationRead,
  sendFirstDirectTextMessage,
  type ConversationMessageReference,
  type CustomerInboxConversationData,
  type CustomerServiceSession,
  type DirectInboxConversationData,
  type GroupInboxConversationData,
  type InboxAssignee,
  type InboxConversation,
  type LoadInboxQuery,
  type MemberOption,
} from "@/api"
import { PageSplit } from "@/components/page-split"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useUserTimeZone } from "@/contexts/user-preferences"
import { useWorkspace } from "@/contexts/workspace-context"
import { previousDayKey } from "@/features/inbox/calendar"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import {
  ConversationComposer,
  ConversationComposerUnavailable,
} from "@/features/inbox/conversation-composer"
import { ConversationContextPane } from "@/features/inbox/conversation-context-pane"
import {
  ConversationAvatar,
  ConversationHeader,
} from "@/features/inbox/conversation-header"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import { CreateGroupConversationDialog } from "@/features/inbox/create-group-conversation-dialog"
import { DirectConversationDraftHeader } from "@/features/inbox/direct-conversation-draft-header"
import { DirectConversationPickerDialog } from "@/features/inbox/direct-conversation-picker-dialog"
import {
  useOutgoingConversationMessages,
  type OutgoingConversationDraft,
} from "@/features/inbox/use-outgoing-conversation-messages"
import {
  useIsNarrowViewport,
  useIsWideViewport,
} from "@/hooks/use-narrow-viewport"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

type ConversationSelection =
  | { kind: "direct-draft"; member: MemberOption }
  | { kind: "conversation"; conversation: InboxConversation }

type InternalInboxConversationData =
  | DirectInboxConversationData
  | GroupInboxConversationData

/** 使用与服务端一致的普通字符串 id 倒序。 */
function compareIDsDescending(first: string, second: string) {
  if (first === second) return 0
  return first > second ? -1 : 1
}

/** 统一按最后消息倒序排列，尚无消息的会话沉底。 */
function compareInboxConversations(
  first: InboxConversation,
  second: InboxConversation,
) {
  const firstSummary = isCustomerInboxConversation(first)
    ? first.customer
    : isDirectInboxConversation(first)
      ? first.direct
      : isGroupInboxConversation(first)
        ? first.group
        : null
  const secondSummary = isCustomerInboxConversation(second)
    ? second.customer
    : isDirectInboxConversation(second)
      ? second.direct
      : isGroupInboxConversation(second)
        ? second.group
        : null
  const firstTime = firstSummary?.lastMessageAt
  const secondTime = secondSummary?.lastMessageAt
  if (!firstTime || !secondTime) {
    if (!firstTime && !secondTime)
      return compareIDsDescending(first.id, second.id)
    return firstTime ? -1 : 1
  }
  const timeDifference = Date.parse(secondTime) - Date.parse(firstTime)
  if (timeDifference) return timeDifference
  return compareIDsDescending(first.id, second.id)
}

/** 生成字段完整且可精确失效的收件箱查询。 */
function inboxQuery(
  scope: InboxScope,
  customerView = CustomerInboxView.CustomerInboxViewQueue,
  assigneeIdentityId = "",
): LoadInboxQuery {
  return { scope, customerView, assigneeIdentityId }
}

/** 返回完整查询的稳定标识。 */
function inboxQueryIdentity(query: LoadInboxQuery) {
  return `${query.scope}\u0000${query.customerView}\u0000${query.assigneeIdentityId}`
}

/** 返回客服处理周期当前所属的筛选查询。 */
function customerPlacementQueries(
  status: ServiceSessionStatus,
  assigneeIdentityId: string,
  currentIdentityId: string,
) {
  if (status === ServiceSessionStatus.ServiceSessionStatusClosed) {
    return [
      inboxQuery(
        InboxScope.InboxScopeCustomer,
        CustomerInboxView.CustomerInboxViewClosed,
      ),
    ]
  }
  if (!assigneeIdentityId) {
    return [inboxQuery(InboxScope.InboxScopeCustomer)]
  }
  if (assigneeIdentityId === currentIdentityId) {
    return [
      inboxQuery(
        InboxScope.InboxScopeCustomer,
        CustomerInboxView.CustomerInboxViewMine,
      ),
    ]
  }
  return [
    inboxQuery(
      InboxScope.InboxScopeCustomer,
      CustomerInboxView.CustomerInboxViewCoworkers,
    ),
    inboxQuery(
      InboxScope.InboxScopeCustomer,
      CustomerInboxView.CustomerInboxViewCoworkers,
      assigneeIdentityId,
    ),
  ]
}

/** 把处理周期命令结果合并到当前客户会话摘要。 */
function customerConversationWithServiceSession(
  conversation: CustomerInboxConversationData,
  session: Pick<CustomerServiceSession, "id" | "status" | "assignee">,
): CustomerInboxConversationData {
  return {
    ...conversation,
    customer: {
      ...conversation.customer,
      serviceSessionId: session.id,
      serviceSessionStatus: session.status,
      assignee: session.assignee,
    },
  }
}

const scopes = [
  {
    id: InboxScope.InboxScopeAll,
    labelKey: "scopeAll",
    icon: MessagesSquareIcon,
  },
  {
    id: InboxScope.InboxScopeCustomer,
    labelKey: "scopeCustomer",
    icon: HeadsetIcon,
  },
  {
    id: InboxScope.InboxScopeInternal,
    labelKey: "scopeInternal",
    icon: UsersRoundIcon,
  },
] as const

/** 客服处理状态文案。 */
function sessionStatusLabel(
  status: ServiceSessionStatus,
  t: TFunction<"inbox">,
) {
  switch (status) {
    case ServiceSessionStatus.ServiceSessionStatusOpen:
      return t("sessionStatus.open")
    case ServiceSessionStatus.ServiceSessionStatusClosed:
      return t("sessionStatus.closed")
    default:
      console.warn("未知的客服处理状态", status)
      return ""
  }
}

/** 会话在列表和主区中的显示名。 */
function useConversationName() {
  const { t } = useTranslation("inbox")
  return useCallback(
    (conversation: InboxConversation) => {
      if (isDirectInboxConversation(conversation)) {
        return conversation.direct.peerName.trim() || t("unknownSender")
      }
      if (isCustomerInboxConversation(conversation)) {
        return conversation.customer.contactName?.trim() || t("anonymousVisitor")
      }
      if (isGroupInboxConversation(conversation)) {
        return conversation.group.title.trim() || t("unknownSender")
      }
      return t("unknownSender")
    },
    [t],
  )
}

/** 按会话列表习惯格式化最近消息时间。 */
function useConversationTime() {
  const { t, i18n } = useTranslation("inbox")
  const timeZone = useUserTimeZone()

  return useMemo(() => {
    const locale = i18n.resolvedLanguage
    const relative = new Intl.RelativeTimeFormat(locale, { numeric: "always" })
    const weekday = new Intl.DateTimeFormat(locale, { timeZone, weekday: "short" })
    const monthDay = new Intl.DateTimeFormat(locale, {
      timeZone,
      month: "numeric",
      day: "numeric",
    })
    const fullDate = new Intl.DateTimeFormat(locale, {
      timeZone,
      year: "numeric",
      month: "numeric",
      day: "numeric",
    })
    /* en-CA 固定输出 YYYY-MM-DD，用于用户时区下的同日、昨天和同年比较。 */
    const dayKey = new Intl.DateTimeFormat("en-CA", {
      timeZone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    })

    return (value: string | null) => {
      if (!value) return ""
      const date = new Date(value)
      const now = new Date()
      const elapsedMs = now.getTime() - date.getTime()
      if (elapsedMs < 60_000) {
        return t("justNow")
      }
      if (elapsedMs < 3_600_000) {
        return relative.format(-Math.floor(elapsedMs / 60_000), "minute")
      }
      const day = dayKey.format(date)
      if (day === dayKey.format(now)) {
        return relative.format(-Math.floor(elapsedMs / 3_600_000), "hour")
      }
      if (day === previousDayKey(dayKey.format(now))) {
        return t("yesterday")
      }
      if (elapsedMs < 6 * 86_400_000) {
        return weekday.format(date)
      }
      if (day.slice(0, 4) === dayKey.format(now).slice(0, 4)) {
        return monthDay.format(date)
      }
      return fullDate.format(date)
    }
  }, [i18n.resolvedLanguage, t, timeZone])
}

/** 每分钟触发一次重渲染，保持相对时间新鲜。 */
function useMinuteTick() {
  const [, setTick] = useState(0)
  useEffect(() => {
    const timer = window.setInterval(() => setTick((tick) => tick + 1), 60_000)
    return () => window.clearInterval(timer)
  }, [])
}

/** 顶部操作行：收纳范围栏和发起会话菜单。 */
function InboxPaneTop({
  railCollapsed,
  onRailToggle,
  onStartDirect,
  onCreateGroup,
}: {
  railCollapsed: boolean
  onRailToggle: () => void
  onStartDirect: () => void
  onCreateGroup: () => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <div
      data-slot="inbox-pane-header"
      className="flex h-14 shrink-0 items-center gap-2 border-b border-border/60 px-3"
    >
      <Button
        variant="ghost"
        size="icon"
        className="shrink-0 text-muted-foreground"
        aria-pressed={railCollapsed}
        aria-label={railCollapsed ? t("scopeRailExpand") : t("scopeRailCollapse")}
        title={railCollapsed ? t("scopeRailExpand") : t("scopeRailCollapse")}
        onClick={onRailToggle}
      >
        <PanelLeftIcon className="size-5" />
      </Button>
      <div className="flex-1" />
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0 bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground"
            aria-label={t("newConversation")}
            title={t("newConversation")}
          >
            <PlusIcon />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="min-w-52">
          <DropdownMenuItem className="gap-2" onSelect={onStartDirect}>
            <span className="min-w-0 flex-1 truncate">
              {t("newDirectConversation")}
            </span>
          </DropdownMenuItem>
          <DropdownMenuItem className="gap-2" onSelect={onCreateGroup}>
            <span className="min-w-0 flex-1 truncate">
              {t("newGroupConversation")}
            </span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}

/** 中栏左缘的范围纵栏。 */
function InboxScopeRail({
  scope,
  onScopeChange,
}: {
  scope: InboxScope
  onScopeChange: (scope: InboxScope) => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <nav
      aria-label={t("scopeRailLabel")}
      className="flex w-20 shrink-0 flex-col gap-0.5 overflow-y-auto border-r border-border/70 bg-muted/30 px-1.5 py-1.5"
    >
      {scopes.map((item) => (
        <button
          key={item.id}
          type="button"
          aria-pressed={scope === item.id}
          className={cn(
            "flex flex-col items-center gap-0.5 rounded-lg px-0 pt-1.5 pb-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
            scope === item.id &&
              "bg-accent font-medium text-accent-foreground hover:bg-accent hover:text-accent-foreground",
          )}
          onClick={() => onScopeChange(item.id)}
        >
          <item.icon className="size-5" />
          <span className="w-full truncate px-px text-center text-xs leading-tight">
            {t(item.labelKey)}
          </span>
        </button>
      ))}
    </nav>
  )
}

/** 客户范围的四个服务视图；同事视图在下拉中继续选择具体客服。 */
function InboxCustomerQueueFilter({
  view,
  assigneeIdentityId,
  assignees,
  currentIdentityId,
  onChange,
}: {
  view: CustomerInboxView
  assigneeIdentityId: string
  assignees: InboxAssignee[]
  currentIdentityId: string
  onChange: (view: CustomerInboxView, assigneeIdentityId?: string) => void
}) {
  const { t } = useTranslation("inbox")
  const coworkers = assignees.filter(
    (assignee) => assignee.identityId !== currentIdentityId,
  )
  const selectedCoworker = coworkers.find(
    (assignee) => assignee.identityId === assigneeIdentityId,
  )
  const segments = [
    {
      id: CustomerInboxView.CustomerInboxViewQueue,
      label: t("queueFilterQueue"),
    },
    {
      id: CustomerInboxView.CustomerInboxViewMine,
      label: t("queueFilterMine"),
    },
  ] as const

  function tabClass(active: boolean) {
    return cn(
      "relative h-9 min-w-0 flex-1 rounded-md px-1 text-center text-sm font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring",
      active
        ? "text-foreground"
        : "text-muted-foreground hover:text-foreground",
    )
  }

  function activeIndicator(active: boolean) {
    return active ? (
      <span
        aria-hidden="true"
        className="absolute right-1 -bottom-px left-1 h-0.5 rounded bg-primary"
      />
    ) : null
  }

  return (
    <div
      role="tablist"
      aria-label={t("queueFilterLabel")}
      className="flex shrink-0 items-stretch border-b border-border/60 px-2 pt-2"
    >
      {segments.map((segment) => {
        const active = view === segment.id
        return (
          <button
            key={segment.id}
            type="button"
            role="tab"
            aria-selected={active}
            className={tabClass(active)}
            onClick={() => onChange(segment.id)}
          >
            <span className="block truncate">{segment.label}</span>
            {activeIndicator(active)}
          </button>
        )
      })}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            role="tab"
            aria-selected={
              view === CustomerInboxView.CustomerInboxViewCoworkers
            }
            title={selectedCoworker?.displayName ?? t("queueFilterColleague")}
            className={tabClass(
              view === CustomerInboxView.CustomerInboxViewCoworkers,
            )}
          >
            <span className="flex min-w-0 items-center justify-center gap-0.5">
              <span className="truncate">{t("queueFilterColleague")}</span>
              <ChevronDownIcon className="size-3.5 opacity-70" />
            </span>
            {activeIndicator(
              view === CustomerInboxView.CustomerInboxViewCoworkers,
            )}
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="center" className="min-w-48">
          <DropdownMenuItem
            className={cn(
              view === CustomerInboxView.CustomerInboxViewCoworkers &&
                !assigneeIdentityId &&
                "bg-muted text-foreground",
            )}
            onSelect={() =>
              onChange(CustomerInboxView.CustomerInboxViewCoworkers, "")
            }
          >
            <span className="min-w-0 flex-1 truncate">
              {t("queueFilterAllCoworkers")}
            </span>
            {view === CustomerInboxView.CustomerInboxViewCoworkers &&
            !assigneeIdentityId ? (
              <CheckIcon className="size-4" />
            ) : null}
          </DropdownMenuItem>
          {coworkers.map((assignee) => (
            <DropdownMenuItem
              key={assignee.identityId}
              className={cn(
                assigneeIdentityId === assignee.identityId &&
                  "bg-muted text-foreground",
              )}
              onSelect={() =>
                onChange(
                  CustomerInboxView.CustomerInboxViewCoworkers,
                  assignee.identityId,
                )
              }
            >
              <span className="min-w-0 flex-1 truncate">
                {assignee.displayName}
              </span>
              {assignee.type ===
              OrganizationIdentityType.OrganizationIdentityTypeAgent ? (
                <span className="text-xs text-muted-foreground">
                  {t("queueFilterAiEmployee")}
                </span>
              ) : null}
              {assigneeIdentityId === assignee.identityId ? (
                <CheckIcon className="size-4" />
              ) : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <button
        type="button"
        role="tab"
        aria-selected={view === CustomerInboxView.CustomerInboxViewClosed}
        className={tabClass(
          view === CustomerInboxView.CustomerInboxViewClosed,
        )}
        onClick={() => onChange(CustomerInboxView.CustomerInboxViewClosed)}
      >
        <span className="block truncate">{t("queueFilterClosed")}</span>
        {activeIndicator(
          view === CustomerInboxView.CustomerInboxViewClosed,
        )}
      </button>
    </div>
  )
}

/** 会话列表。 */
function InboxConversationList({
  conversations,
  loading,
  selectedId,
  onSelect,
  onMarkRead,
}: {
  conversations: InboxConversation[]
  loading: boolean
  selectedId?: string
  onSelect: (conversationId: string) => void
  onMarkRead: (conversation: InboxConversation) => void
}) {
  const { t } = useTranslation("inbox")
  const conversationName = useConversationName()
  const formatTime = useConversationTime()
  useMinuteTick()

  if (loading) {
    return (
      <LoadingIndicator className="min-h-0 flex-1 justify-center">
        {t("messagesLoading")}
      </LoadingIndicator>
    )
  }

  if (conversations.length === 0) return null

  return (
    <ScrollArea className="min-h-0 min-w-0 flex-1 [&>[data-slot=scroll-area-viewport]>div]:block">
      <div className="grid min-w-0 pb-1.5">
        {conversations.map((conversation) => {
          const name = conversationName(conversation)
          const summary = isCustomerInboxConversation(conversation)
            ? conversation.customer
            : isDirectInboxConversation(conversation)
              ? conversation.direct
              : isGroupInboxConversation(conversation)
                ? conversation.group
                : null
          if (!summary) return null
          const agentRunLabel = agentRunStatusLabel(
            isDirectInboxConversation(conversation)
              ? conversation.direct.agentRunStatus
              : null,
            t,
          )
          const groupDissolved =
            isGroupInboxConversation(conversation) &&
            conversation.group.status ===
              ConversationStatus.ConversationStatusArchived
          const preview =
            groupDissolved
              ? t("groupDissolved")
              : summary.preview?.trim() ||
                (isGroupInboxConversation(conversation) &&
                summary.lastMessageAt
                  ? t("groupSystemUpdated")
                  : t("messagesEmpty"))
          const formattedTime = formatTime(summary.lastMessageAt)
          const hasUnread = conversation.unreadCount > 0
          return (
            <ContextMenu key={conversation.id}>
              <ContextMenuTrigger asChild>
                <button
              type="button"
              aria-pressed={selectedId === conversation.id}
              aria-label={name}
              className={cn(
                "flex w-full min-w-0 items-start gap-3 px-3 py-2.5 text-left transition-colors",
                selectedId === conversation.id
                  ? "bg-accent text-accent-foreground"
                  : "hover:bg-muted",
              )}
              onClick={() => onSelect(conversation.id)}
                >
              <span className="relative shrink-0">
                <ConversationAvatar conversation={conversation} />
                {hasUnread ? (
                  <span className="absolute -top-1.5 -right-1.5 flex min-w-5 items-center justify-center gap-0.5 rounded-full bg-destructive px-1 text-[10px] font-semibold leading-5 text-destructive-foreground ring-2 ring-background">
                    {conversation.mentionedUnreadCount > 0 ? "@" : null}
                    {conversation.unreadCount > 99
                      ? "99+"
                      : conversation.unreadCount}
                  </span>
                ) : null}
              </span>
              <span className="min-w-0 flex-1 overflow-hidden">
                <span className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
                  <span className="flex min-w-0 items-center gap-2">
                    <span className="min-w-0 flex-1 truncate text-sm font-medium">
                      {name}
                    </span>
                    {agentRunLabel ? (
                      <span className="shrink-0 text-[10px] text-muted-foreground">
                        {agentRunLabel}
                      </span>
                    ) : null}
                  </span>
                  <span className="flex shrink-0 items-center gap-1.5">
                    {formattedTime ? (
                      <time
                        dateTime={summary.lastMessageAt ?? undefined}
                        className={cn(
                          "shrink-0 text-xs text-muted-foreground",
                          selectedId === conversation.id &&
                            "text-accent-foreground/75",
                        )}
                      >
                        {formattedTime}
                      </time>
                    ) : null}
                  </span>
                </span>
                <span
                  title={preview}
                  className={cn(
                    "mt-0.5 block w-full min-w-0 truncate text-xs text-muted-foreground",
                    selectedId === conversation.id &&
                      "text-accent-foreground/75",
                  )}
                >
                  {preview}
                </span>
              </span>
                </button>
              </ContextMenuTrigger>
              {hasUnread && conversation.lastMessageId ? (
                <ContextMenuContent>
                  <ContextMenuItem onSelect={() => onMarkRead(conversation)}>
                    {t("conversationMarkRead")}
                  </ContextMenuItem>
                </ContextMenuContent>
              ) : null}
            </ContextMenu>
          )
        })}
      </div>
    </ScrollArea>
  )
}

/** 组合当前 Conversation 工作区和独立联系人上下文栏。 */
function ConversationMain({
  selection,
  onSessionMoved,
  onConversationChanged,
  onGroupLeft,
  onDirectStarted,
  narrowViewport = false,
}: {
  selection: ConversationSelection
  onSessionMoved: (
    conversation: CustomerInboxConversationData,
    session: CustomerServiceSession,
    view: CustomerInboxView,
    assigneeIdentityId?: string,
  ) => void
  onConversationChanged: (
    conversation:
      | CustomerInboxConversationData
      | DirectInboxConversationData
      | GroupInboxConversationData,
  ) => void
  onGroupLeft: (conversationID: string) => void
  onDirectStarted: (conversation: DirectInboxConversationData) => void
  narrowViewport?: boolean
}) {
  const conversation =
    selection.kind === "conversation" ? selection.conversation : null
  const directTarget =
    selection.kind === "direct-draft" ? selection.member : null
  const { t } = useTranslation("inbox")
  const { identity } = useWorkspace()
  const isWideViewport = useIsWideViewport()
  const conversationName = useConversationName()
  const [contextCollapsed, setContextCollapsed] = useState(
    () => !isWideViewport,
  )

  useEffect(() => {
    // 跨过响应式断点时恢复当前宽度对应的默认状态。
    setContextCollapsed(!isWideViewport)
  }, [isWideViewport])

  const sourceGroupConversation =
    conversation && isGroupInboxConversation(conversation) ? conversation : null
  const [groupSummaryDraft, setGroupSummaryDraft] = useState<{
    conversationID: string
    summary: GroupInboxConversationData["group"]
  } | null>(null)
  const activeGroupSummary = sourceGroupConversation
    ? groupSummaryDraft?.conversationID === sourceGroupConversation.id
      ? groupSummaryDraft.summary
      : sourceGroupConversation.group
    : null

  const displayedConversation =
    sourceGroupConversation && activeGroupSummary
      ? { ...sourceGroupConversation, group: activeGroupSummary }
      : conversation
  const contactName = displayedConversation
    ? conversationName(displayedConversation)
    : directTarget?.displayName ?? ""
  const customerConversation =
    displayedConversation && isCustomerInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const directConversation =
    displayedConversation && isDirectInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const groupConversation =
    displayedConversation && isGroupInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const sessionStatus = customerConversation
    ? sessionStatusLabel(
        customerConversation.customer.serviceSessionStatus,
        t,
      )
    : ""
  const replyDisabledReason = customerConversation
    ? customerConversation.customer.serviceSessionStatus ===
      ServiceSessionStatus.ServiceSessionStatusClosed
      ? t("replyClosedUnavailable")
      : customerConversation.customer.assignee &&
          customerConversation.customer.assignee.identityId !==
            identity.user.identityId
        ? t("replyAssignedUnavailable", {
            name: customerConversation.customer.assignee.displayName,
          })
        : null
    : groupConversation?.group.status ===
        ConversationStatus.ConversationStatusArchived
      ? t("groupDissolvedUnavailable")
      : null
  const validConversation =
    customerConversation ?? directConversation ?? groupConversation
  if (!validConversation && !directTarget) return null
  // 单聊按目标身份保持消息组件，首发落库不会清空失败消息和输入状态。
  const threadKey =
    directTarget?.id ?? directConversation?.direct.peerIdentityId ?? validConversation?.id

  return (
    <div className="flex h-full min-h-0 bg-background">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        {validConversation ? (
          <ConversationHeader
            conversation={validConversation}
            contactName={contactName}
            sessionStatus={sessionStatus}
            currentIdentityId={identity.user.identityId}
            onSessionMoved={(session, view, assigneeIdentityId) => {
              if (!customerConversation) return
              onSessionMoved(
                customerConversation,
                session,
                view,
                assigneeIdentityId,
              )
            }}
            narrowViewport={narrowViewport}
          />
        ) : directTarget ? (
          <DirectConversationDraftHeader member={directTarget} />
        ) : null}
        <ConversationThread
          key={threadKey}
          conversation={validConversation}
          directTarget={directTarget}
          replyDisabledReason={replyDisabledReason}
          onConversationChanged={() => {
            if (validConversation) onConversationChanged(validConversation)
          }}
          onDirectStarted={onDirectStarted}
        />
      </div>
      <ConversationContextPane
        conversation={validConversation}
        directTarget={directTarget}
        displayName={contactName}
        currentIdentityID={identity.user.identityId}
        onGroupSummaryChange={(changes) => {
          if (!sourceGroupConversation) return
          setGroupSummaryDraft((current) => ({
            conversationID: sourceGroupConversation.id,
            summary: {
              ...(current?.conversationID === sourceGroupConversation.id
                ? current.summary
                : sourceGroupConversation.group),
              ...changes,
            },
          }))
        }}
        onGroupLeft={() => {
          if (validConversation) onGroupLeft(validConversation.id)
        }}
        visible={!contextCollapsed}
        onToggle={() => setContextCollapsed((collapsed) => !collapsed)}
      />
    </div>
  )
}

/** 协调当前会话时间线和回复区的即时消息。 */
function ConversationThread({
  conversation,
  directTarget,
  replyDisabledReason,
  onConversationChanged,
  onDirectStarted,
}: {
  conversation:
    | CustomerInboxConversationData
    | DirectInboxConversationData
    | GroupInboxConversationData
    | null
  directTarget: MemberOption | null
  replyDisabledReason: string | null
  onConversationChanged: () => void
  onDirectStarted: (conversation: DirectInboxConversationData) => void
}) {
  const { t } = useTranslation("inbox")
  const { identity } = useWorkspace()
  const outgoing = useOutgoingConversationMessages()
  const invalidate = useResourceInvalidator()
  const aliveRef = useRef(true)
  const conversationID = conversation?.id ?? ""
  const conversationType =
    conversation?.type ?? ConversationType.ConversationTypeDirect

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
    }
  }, [])
  const [replyTo, setReplyTo] = useState<ConversationMessageReference | null>(
    null,
  )
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const messageSending = outgoing.messages.some(
    (message) => message.status === "sending",
  )
  const replySupported =
    !conversation ||
    isDirectInboxConversation(conversation) ||
    isGroupInboxConversation(conversation) ||
    conversation.customer.channelType === ChannelType.ChannelTypeWebsite
  const groupConversation =
    conversation && isGroupInboxConversation(conversation) ? conversation : null
  const groupResource = useResource(
    resourceKeys.groupConversation(groupConversation?.id ?? ""),
    () => getGroupConversation(groupConversation?.id ?? ""),
    { enabled: Boolean(groupConversation) },
  )

  /** 保存当前已看到的最新内部消息并刷新收件箱未读摘要。 */
  const markRead = useCallback(
    (messageID: string) => {
      if (!conversation || isCustomerInboxConversation(conversation)) return
      void markConversationRead(conversation.id, {
        lastReadMessageId: messageID,
      })
        .then(() => onConversationChanged())
        .catch((error: unknown) =>
          console.warn("标记会话已读失败", {
            conversationId: conversation.id,
            error,
          }),
        )
    },
    [conversation, onConversationChanged],
  )

  return (
    <>
      <ConversationTimeline
        conversationID={conversationID}
        conversationType={conversationType}
        currentIdentityID={identity.user.identityId}
        workspaceLayout
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={setRetryDraft}
        retryFailedMessageDisabled={
          messageSending || !replySupported || Boolean(replyDisabledReason)
        }
        groupParticipants={groupResource.data?.participants}
        onReplyMessage={
          groupConversation && !replyDisabledReason
            ? setReplyTo
            : undefined
        }
        onReadMessage={
          !conversation || isCustomerInboxConversation(conversation)
            ? undefined
            : markRead
        }
        readThroughMessageID={
          !conversation || isCustomerInboxConversation(conversation)
            ? undefined
            : conversation.lastReadMessageId
        }
        enabled={Boolean(conversation)}
      />
      {!replySupported || replyDisabledReason ? (
        <ConversationComposerUnavailable
          conversationID={conversationID}
          reason={replyDisabledReason ?? t("channelReplyUnsupported")}
        />
      ) : (
        <ConversationComposer
          conversationID={conversationID}
          conversationType={conversationType}
          submitOnEnter
          refocusAfterSubmit
          retryFailedMessage
          retryDraft={retryDraft}
          replyTo={replyTo}
          groupParticipants={groupResource.data?.participants}
          currentIdentityID={identity.user.identityId}
          onRetryDraftHandled={() => setRetryDraft(null)}
          onReplyToChange={setReplyTo}
          onSending={outgoing.start}
          onSent={outgoing.succeed}
          onFailed={outgoing.fail}
          onSucceeded={onConversationChanged}
          sendDirectMessage={
            directTarget
              ? async (input) => {
                  const result = await sendFirstDirectTextMessage({
                    targetIdentityId: directTarget.id,
                    ...input,
                  })
                  void invalidate(
                    resourceKeys.directConversation(directTarget.id),
                  )
                  void invalidate(
                    resourceKeys.conversationMessages(result.conversation.id),
                  )
                  // 离开原线程后只刷新列表，不改变当前选择。
                  if (aliveRef.current) {
                    onDirectStarted(result.conversation)
                  } else {
                    void invalidate(resourceKeys.inbox())
                  }
                  return result.message
                }
              : undefined
          }
        />
      )}
    </>
  )
}

/** 消息页中栏和当前会话。 */
export function InboxPage({
  conversations,
  listLoading,
  listError,
  onListRefresh,
  scope,
  customerView,
  assigneeIdentityId,
  selectedConversationId,
  onSelectedConversationChange,
  onQueryChange,
}: {
  conversations: InboxConversation[]
  listLoading: boolean
  listError: boolean
  onListRefresh: () => void
  scope: InboxScope
  customerView: CustomerInboxView
  assigneeIdentityId: string
  selectedConversationId: string
  onSelectedConversationChange: (
    conversationId: string,
    replace?: boolean,
  ) => void
  onQueryChange: (changes: {
    scope?: InboxScope
    customerView?: CustomerInboxView
    assigneeIdentityId?: string
    conversationId?: string
    replace?: boolean
  }) => void
}) {
  const { t } = useTranslation("inbox")
  const { identity } = useWorkspace()
  const isNarrowViewport = useIsNarrowViewport()
  const invalidate = useResourceInvalidator()
  const [railCollapsed, setRailCollapsed] = useState(false)
  const [directDraft, setDirectDraft] = useState<MemberOption | null>(null)
  const [isNarrowDetailOpen, setIsNarrowDetailOpen] = useState(false)
  const [directDialogOpen, setDirectDialogOpen] = useState(false)
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const [startedConversations, setStartedConversations] = useState<
    InternalInboxConversationData[]
  >([])
  const [leftGroupConversationIDs, setLeftGroupConversationIDs] = useState<
    Set<string>
  >(new Set())
  const [selectedConversationSnapshot, setSelectedConversationSnapshot] =
    useState<InboxConversation | null>(null)
  const conversationName = useConversationName()
  const currentInboxQuery = inboxQuery(
    scope,
    customerView,
    assigneeIdentityId,
  )
  const { data: customerServiceAssignees = [] } = useResource(
    resourceKeys.customerServiceAssignees(),
    () => listCustomerServiceAssignees(),
    { enabled: scope === InboxScope.InboxScopeCustomer },
  )

  const validConversations = useMemo(
    () =>
      conversations.filter(
        (conversation) =>
          !leftGroupConversationIDs.has(conversation.id) &&
          (isCustomerInboxConversation(conversation) ||
            isDirectInboxConversation(conversation) ||
            isGroupInboxConversation(conversation)),
      ),
    [conversations, leftGroupConversationIDs],
  )
  const allConversations = useMemo(
    () =>
      [
        ...startedConversations.filter(
          (started) =>
            !validConversations.some(
              (conversation) => conversation.id === started.id,
            ),
        ),
        ...validConversations,
      ].sort(compareInboxConversations),
    [startedConversations, validConversations],
  )
  const scopedConversations = useMemo(() => {
    switch (scope) {
      case InboxScope.InboxScopeCustomer:
        return allConversations.filter(isCustomerInboxConversation)
      case InboxScope.InboxScopeInternal:
        return allConversations.filter(
          (conversation) =>
            isDirectInboxConversation(conversation) ||
            isGroupInboxConversation(conversation),
        )
      default:
        return allConversations
    }
  }, [allConversations, scope])

  useEffect(() => {
    if (!isNarrowViewport) {
      setIsNarrowDetailOpen(false)
    }
  }, [isNarrowViewport])

  useEffect(() => {
    setStartedConversations((current) => {
      const pending = current.filter(
        (started) =>
          !validConversations.some(
            (conversation) => conversation.id === started.id,
          ),
      )
      return pending.length === current.length ? current : pending
    })
  }, [validConversations])

  useEffect(() => {
    const listedIDs = new Set(
      conversations.map((conversation) => conversation.id),
    )
    setLeftGroupConversationIDs((current) => {
      const pending = new Set(
        [...current].filter((conversationID) =>
          listedIDs.has(conversationID),
        ),
      )
      return pending.size === current.size ? current : pending
    })
  }, [conversations])

  const activeDirectDraft =
    scope === InboxScope.InboxScopeInternal && !selectedConversationId
      ? directDraft
      : null
  useEffect(() => {
    if (selectedConversationId || scope !== InboxScope.InboxScopeInternal) {
      setDirectDraft(null)
    }
  }, [scope, selectedConversationId])
  const selectedPool = listLoading ? allConversations : scopedConversations
  const selectedFromPool = activeDirectDraft
    ? undefined
    : selectedPool.find(
        (conversation) => conversation.id === selectedConversationId,
      ) ?? (selectedConversationId ? undefined : selectedPool[0])
  useEffect(() => {
    if (selectedFromPool) setSelectedConversationSnapshot(selectedFromPool)
  }, [selectedFromPool])
  useEffect(() => {
    if (
      activeDirectDraft ||
      listLoading ||
      (selectedConversationId &&
        scopedConversations.some(
          (conversation) => conversation.id === selectedConversationId,
        ))
    ) {
      return
    }
    const nextConversationID = scopedConversations[0]?.id ?? ""
    if (nextConversationID === selectedConversationId) return
    setSelectedConversationSnapshot(null)
    onSelectedConversationChange(nextConversationID, true)
  }, [
    activeDirectDraft,
    listLoading,
    onSelectedConversationChange,
    scopedConversations,
    selectedConversationId,
  ])
  const selectedConversation = activeDirectDraft
    ? undefined
    : selectedConversationSnapshot?.id === selectedConversationId &&
      selectedPool.some(
        (conversation) => conversation.id === selectedConversationId,
      )
        ? selectedConversationSnapshot
        : selectedFromPool

  /** 切入另一条内部会话时直接把当前最后消息标为已读。 */
  useEffect(() => {
    if (
      !selectedConversation ||
      (!isDirectInboxConversation(selectedConversation) &&
        !isGroupInboxConversation(selectedConversation)) ||
      !selectedConversation.lastMessageId ||
      selectedConversation.unreadCount === 0
    ) {
      return
    }
    void markConversationRead(selectedConversation.id, {
      lastReadMessageId: selectedConversation.lastMessageId,
    })
      .then(() => refreshConversationAfterMessage(selectedConversation))
      .catch((error: unknown) =>
        console.warn("进入会话时标记已读失败", {
          conversationId: selectedConversation.id,
          error,
        }),
      )
  }, [selectedConversation?.id])

  /** 选中一个会话。 */
  function selectConversation(conversationId: string) {
    setDirectDraft(null)
    onSelectedConversationChange(conversationId)

    if (isNarrowViewport) {
      setIsNarrowDetailOpen(true)
    }
  }

  /** 不打开会话并把列表项推进到当前最后消息。 */
  function markConversationAsRead(conversation: InboxConversation) {
    if (
      (!isDirectInboxConversation(conversation) &&
        !isGroupInboxConversation(conversation)) ||
      !conversation.lastMessageId ||
      conversation.unreadCount === 0
    ) {
      return
    }
    void markConversationRead(conversation.id, {
      lastReadMessageId: conversation.lastMessageId,
    })
      .then(() => refreshConversationAfterMessage(conversation))
      .catch((error: unknown) =>
        console.warn("标记列表会话已读失败", {
          conversationId: conversation.id,
          error,
        }),
      )
  }

  /** 标旧受影响的精确查询，仅让最终可见视图立即重新读取。 */
  function refreshAffectedInboxQueries(
    queries: LoadInboxQuery[],
    destination: LoadInboxQuery,
    nextConversationId?: string,
  ) {
    const destinationIdentity = inboxQueryIdentity(destination)
    const affected = new Map(
      queries.map((query) => [inboxQueryIdentity(query), query]),
    )
    affected.set(destinationIdentity, destination)
    for (const [identity, query] of affected) {
      if (identity === destinationIdentity) continue
      void invalidate(resourceKeys.inbox(query), {
        exact: true,
        refetchType: "none",
      })
    }
    void invalidate(resourceKeys.inbox(destination), { exact: true })
    if (destinationIdentity === inboxQueryIdentity(currentInboxQuery)) {
      if (nextConversationId) {
        onSelectedConversationChange(nextConversationId)
      }
      return
    }
    onQueryChange({
      scope: destination.scope,
      customerView: destination.customerView,
      assigneeIdentityId: destination.assigneeIdentityId,
      conversationId: nextConversationId,
      replace: nextConversationId ? false : undefined,
    })
  }

  /** 将新建或新打开的内部会话放入列表并选中。 */
  function showStartedConversation(
    conversation: InternalInboxConversationData,
  ) {
    setDirectDraft(null)
    setStartedConversations((current) => [
      conversation,
      ...current.filter((item) => item.id !== conversation.id),
    ])
    const destination = inboxQuery(InboxScope.InboxScopeInternal)
    refreshAffectedInboxQueries(
      [inboxQuery(InboxScope.InboxScopeAll), destination],
      destination,
      conversation.id,
    )
    setIsNarrowDetailOpen(isNarrowViewport)
  }

  /** 在主区打开不持久化的单聊草稿。 */
  function showDirectDraft(
    member: MemberOption,
    existing: DirectInboxConversationData | null,
  ) {
    if (existing) {
      showStartedConversation(existing)
      return
    }
    setDirectDraft(member)
    onQueryChange({
      scope: InboxScope.InboxScopeInternal,
      conversationId: "",
    })
    setIsNarrowDetailOpen(isNarrowViewport)
  }

  /** 会话命令改变归属后刷新源、目标与全部视图。 */
  function showMovedCustomerConversation(
    conversation: CustomerInboxConversationData,
    session: CustomerServiceSession,
    view: CustomerInboxView,
    nextAssigneeIdentityId = "",
  ) {
    setSelectedConversationSnapshot(
      customerConversationWithServiceSession(conversation, session),
    )
    const destination =
      scope === InboxScope.InboxScopeCustomer ||
      view === CustomerInboxView.CustomerInboxViewClosed
        ? inboxQuery(
            InboxScope.InboxScopeCustomer,
            view,
            nextAssigneeIdentityId,
          )
        : currentInboxQuery
    refreshAffectedInboxQueries(
      [
        inboxQuery(InboxScope.InboxScopeAll),
        ...customerPlacementQueries(
          conversation.customer.serviceSessionStatus,
          conversation.customer.assignee?.identityId ?? "",
          identity.user.identityId,
        ),
        ...customerPlacementQueries(
          session.status,
          session.assignee?.identityId ?? "",
          identity.user.identityId,
        ),
      ],
      destination,
    )
  }

  /** 回复成功后只刷新会话会出现或排序变化的精确视图。 */
  function refreshConversationAfterMessage(
    conversation:
      | CustomerInboxConversationData
      | DirectInboxConversationData
      | GroupInboxConversationData,
  ) {
    if (
      isDirectInboxConversation(conversation) ||
      isGroupInboxConversation(conversation)
    ) {
      refreshAffectedInboxQueries(
        [
          inboxQuery(InboxScope.InboxScopeAll),
          inboxQuery(InboxScope.InboxScopeInternal),
        ],
        currentInboxQuery,
      )
      return
    }
    const implicitlyClaimed = !conversation.customer.assignee
    if (implicitlyClaimed) {
      setSelectedConversationSnapshot(
        customerConversationWithServiceSession(conversation, {
          id: conversation.customer.serviceSessionId,
          status: ServiceSessionStatus.ServiceSessionStatusOpen,
          assignee: {
            identityId: identity.user.identityId,
            type: OrganizationIdentityType.OrganizationIdentityTypeUser,
            displayName: identity.user.displayName,
            avatarUrl: identity.user.avatarUrl,
          },
        }),
      )
    }
    const destination =
      implicitlyClaimed && scope === InboxScope.InboxScopeCustomer
        ? inboxQuery(
            InboxScope.InboxScopeCustomer,
            CustomerInboxView.CustomerInboxViewMine,
          )
        : currentInboxQuery
    refreshAffectedInboxQueries(
      [
        inboxQuery(InboxScope.InboxScopeAll),
        ...customerPlacementQueries(
          conversation.customer.serviceSessionStatus,
          conversation.customer.assignee?.identityId ?? "",
          identity.user.identityId,
        ),
        ...(implicitlyClaimed
          ? customerPlacementQueries(
              ServiceSessionStatus.ServiceSessionStatusOpen,
              identity.user.identityId,
              identity.user.identityId,
            )
          : []),
      ],
      destination,
    )
  }

  /** 群聊退出后立即切换到下一条仍可访问的会话。 */
  function showConversationAfterGroupLeft(conversationID: string) {
    setLeftGroupConversationIDs((current) =>
      new Set(current).add(conversationID),
    )
    setStartedConversations((current) =>
      current.filter((conversation) => conversation.id !== conversationID),
    )
    setSelectedConversationSnapshot(null)
    setIsNarrowDetailOpen(false)
    const nextConversationID = scopedConversations.find(
      (conversation) => conversation.id !== conversationID,
    )?.id
    onSelectedConversationChange(nextConversationID ?? "", true)
  }

  const selection: ConversationSelection | null = activeDirectDraft
    ? { kind: "direct-draft", member: activeDirectDraft }
    : selectedConversation
      ? { kind: "conversation", conversation: selectedConversation }
      : null

  const pane = (
    <div className="flex min-h-0 flex-1 flex-col">
      <InboxPaneTop
        railCollapsed={railCollapsed}
        onRailToggle={() => setRailCollapsed((collapsed) => !collapsed)}
        onStartDirect={() => setDirectDialogOpen(true)}
        onCreateGroup={() => setGroupDialogOpen(true)}
      />
      {listError ? (
        <button
          type="button"
          className="min-h-9 w-full shrink-0 border-b bg-warning/10 px-3 py-2 text-center text-xs text-warning"
          onClick={onListRefresh}
        >
          {t("inboxRefreshError")}
        </button>
      ) : null}
      <div className="flex min-h-0 flex-1">
        {railCollapsed ? null : (
          <InboxScopeRail
            scope={scope}
            onScopeChange={(nextScope) => {
              setDirectDraft(null)
              onQueryChange({ scope: nextScope })
            }}
          />
        )}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
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
          <InboxConversationList
            conversations={scopedConversations}
            loading={listLoading}
            selectedId={selectedConversationId}
            onSelect={selectConversation}
            onMarkRead={markConversationAsRead}
          />
        </div>
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
        {isNarrowViewport ? null : selection ? (
          <section className="min-h-0 flex-1">
            <ConversationMain
              selection={selection}
              onSessionMoved={showMovedCustomerConversation}
              onConversationChanged={refreshConversationAfterMessage}
              onGroupLeft={showConversationAfterGroupLeft}
              onDirectStarted={showStartedConversation}
            />
          </section>
        ) : (
          <div className="cervi-inbox-empty-main flex min-h-0 flex-1 items-center justify-center p-6">
            <div data-slot="empty-state-content" className="max-w-sm text-center">
              <div className="mx-auto mb-4 flex size-11 items-center justify-center rounded-xl border bg-background shadow-sm">
                <MessagesSquareIcon className="size-5 text-muted-foreground" />
              </div>
              <h2 className="text-base font-semibold tracking-tight">
                {t("emptyTitle")}
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                {t("emptyDescription")}
              </p>
            </div>
          </div>
        )}
      </PageSplit>

      {selection ? (
        <Sheet
          open={isNarrowDetailOpen}
          onOpenChange={(open) => {
            setIsNarrowDetailOpen(open)
            if (!open) setDirectDraft(null)
          }}
        >
          <SheetContent className="data-[side=right]:w-full p-0 sm:max-w-lg">
            <SheetHeader className="sr-only">
              <SheetTitle>
                {t("conversationTitle", {
                  name: activeDirectDraft?.displayName ?? (
                    selectedConversation ? conversationName(selectedConversation) : ""
                  ),
                })}
              </SheetTitle>
              <SheetDescription>{t("detailDescription")}</SheetDescription>
            </SheetHeader>
            <ConversationMain
              selection={selection}
              onSessionMoved={showMovedCustomerConversation}
              onConversationChanged={refreshConversationAfterMessage}
              onGroupLeft={showConversationAfterGroupLeft}
              onDirectStarted={showStartedConversation}
              narrowViewport
            />
          </SheetContent>
        </Sheet>
      ) : null}

      <DirectConversationPickerDialog
        open={directDialogOpen}
        currentIdentityID={identity.user.identityId}
        onOpenChange={setDirectDialogOpen}
        onSelected={showDirectDraft}
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
