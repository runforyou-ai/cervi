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
  SearchIcon,
  UsersRoundIcon,
  XIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  ChannelType,
  ConversationStatus,
  CustomerInboxView,
  InboxScope,
  OrganizationIdentityType,
  ServiceSessionStatus,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  getGroupConversation,
  listCustomerServiceAssignees,
  type ConversationMessageReference,
  type CustomerInboxConversationData,
  type CustomerServiceSession,
  type DirectInboxConversationData,
  type GroupInboxConversationData,
  type InboxAssignee,
  type InboxConversation,
  type LoadInboxQuery,
} from "@/api"
import { PageSplit } from "@/components/page-split"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
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
import { StartDirectConversationDialog } from "@/features/inbox/start-direct-conversation-dialog"
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

/** 顶部操作行：收纳纵栏、搜索框和发起会话占位菜单。 */
function InboxPaneTop({
  query,
  onQueryChange,
  railCollapsed,
  onRailToggle,
  searchDisabled,
  onStartDirect,
  onCreateGroup,
}: {
  query: string
  onQueryChange: (query: string) => void
  railCollapsed: boolean
  onRailToggle: () => void
  searchDisabled: boolean
  onStartDirect: () => void
  onCreateGroup: () => void
}) {
  const { t } = useTranslation("inbox")
  const searchInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    /** Ctrl/⌘+K 聚焦搜索框。 */
    function focusSearch(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault()
        searchInputRef.current?.focus()
      }
    }
    window.addEventListener("keydown", focusSearch)
    return () => window.removeEventListener("keydown", focusSearch)
  }, [])

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
      <div className="relative min-w-0 flex-1">
        <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          ref={searchInputRef}
          type="text"
          autoComplete="off"
          value={query}
          disabled={searchDisabled}
          aria-label={t("searchLabel")}
          title={t("searchShortcut")}
          className="h-9 w-full rounded-md border border-transparent bg-muted pr-8 pl-8 text-sm text-foreground transition-colors outline-none focus:border-ring focus:bg-background disabled:opacity-50"
          onChange={(event) => onQueryChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Escape" && query) {
              event.stopPropagation()
              onQueryChange("")
            }
          }}
        />
        {query ? (
          <button
            type="button"
            aria-label={t("searchClear")}
            className="absolute top-1/2 right-1.5 flex size-6 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:bg-background hover:text-foreground"
            onClick={() => onQueryChange("")}
          >
            <XIcon className="size-3.5" />
          </button>
        ) : null}
      </div>
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
  hasAnyConversation,
  loading,
  selectedId,
  onSelect,
}: {
  conversations: InboxConversation[]
  hasAnyConversation: boolean
  loading: boolean
  selectedId?: string
  onSelect: (conversationId: string) => void
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

  if (conversations.length === 0) {
    return hasAnyConversation ? (
      <p className="px-6 py-12 text-center text-sm text-muted-foreground">
        {t("searchEmpty")}
      </p>
    ) : null
  }

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
          return (
            <button
              key={conversation.id}
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
              <ConversationAvatar conversation={conversation} />
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
          )
        })}
      </div>
    </ScrollArea>
  )
}

/** 组合当前 Conversation 工作区和独立联系人上下文栏。 */
function ConversationMain({
  conversation,
  onSessionMoved,
  onConversationChanged,
  onGroupLeft,
  narrowViewport = false,
}: {
  conversation: InboxConversation
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
  narrowViewport?: boolean
}) {
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

  const sourceGroupConversation = isGroupInboxConversation(conversation)
    ? conversation
    : null
  const [groupSummaryDraft, setGroupSummaryDraft] = useState<{
    conversationID: string
    summary: GroupInboxConversationData["group"]
  } | null>(null)
  const activeGroupSummary = sourceGroupConversation
    ? groupSummaryDraft?.conversationID === conversation.id
      ? groupSummaryDraft.summary
      : sourceGroupConversation.group
    : null

  const displayedConversation: InboxConversation =
    sourceGroupConversation && activeGroupSummary
      ? { ...sourceGroupConversation, group: activeGroupSummary }
      : conversation
  const contactName = conversationName(displayedConversation)
  const customerConversation = isCustomerInboxConversation(
    displayedConversation,
  )
    ? displayedConversation
    : null
  const directConversation = isDirectInboxConversation(
    displayedConversation,
  )
    ? displayedConversation
    : null
  const groupConversation = isGroupInboxConversation(displayedConversation)
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
  if (!validConversation) return null

  return (
    <div className="flex h-full min-h-0 bg-background">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
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
        <ConversationThread
          key={conversation.id}
          conversation={validConversation}
          replyDisabledReason={replyDisabledReason}
          onConversationChanged={() =>
            onConversationChanged(validConversation)
          }
        />
      </div>
      <ConversationContextPane
        conversation={validConversation}
        displayName={contactName}
        currentIdentityID={identity.user.identityId}
        onGroupSummaryChange={(changes) => {
          if (!sourceGroupConversation) return
          setGroupSummaryDraft((current) => ({
            conversationID: conversation.id,
            summary: {
              ...(current?.conversationID === conversation.id
                ? current.summary
                : sourceGroupConversation.group),
              ...changes,
            },
          }))
        }}
        onGroupLeft={() => onGroupLeft(conversation.id)}
        visible={!contextCollapsed}
        onToggle={() => setContextCollapsed((collapsed) => !collapsed)}
      />
    </div>
  )
}

/** 协调当前会话时间线和回复区的即时消息。 */
function ConversationThread({
  conversation,
  replyDisabledReason,
  onConversationChanged,
}: {
  conversation:
    | CustomerInboxConversationData
    | DirectInboxConversationData
    | GroupInboxConversationData
  replyDisabledReason: string | null
  onConversationChanged: () => void
}) {
  const { t } = useTranslation("inbox")
  const { identity } = useWorkspace()
  const outgoing = useOutgoingConversationMessages()
  const [replyTo, setReplyTo] = useState<ConversationMessageReference | null>(
    null,
  )
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const messageSending = outgoing.messages.some(
    (message) => message.status === "sending",
  )
  const replySupported =
    isDirectInboxConversation(conversation) ||
    isGroupInboxConversation(conversation) ||
    conversation.customer.channelType === ChannelType.ChannelTypeWebsite
  const groupConversation = isGroupInboxConversation(conversation)
    ? conversation
    : null
  const groupResource = useResource(
    resourceKeys.groupConversation(groupConversation?.id ?? ""),
    () => getGroupConversation(groupConversation?.id ?? ""),
    { enabled: Boolean(groupConversation) },
  )

  return (
    <>
      <ConversationTimeline
        conversationID={conversation.id}
        conversationType={conversation.type}
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
      />
      {!replySupported || replyDisabledReason ? (
        <ConversationComposerUnavailable
          conversationID={conversation.id}
          reason={replyDisabledReason ?? t("channelReplyUnsupported")}
        />
      ) : (
        <ConversationComposer
          conversationID={conversation.id}
          conversationType={conversation.type}
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
  const [query, setQuery] = useState("")
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

  const normalizedQuery = query.trim().toLocaleLowerCase()
  const visibleConversations = useMemo(() => {
    if (!normalizedQuery) {
      return scopedConversations
    }
    return scopedConversations.filter((conversation) => {
      const values = isCustomerInboxConversation(conversation)
        ? [
            conversationName(conversation),
            conversation.customer.title,
            conversation.customer.preview ?? "",
            conversation.customer.channelName,
          ]
        : isDirectInboxConversation(conversation)
          ? [
              conversation.direct.peerName,
              conversation.direct.preview ?? "",
            ]
          : isGroupInboxConversation(conversation)
            ? [conversation.group.title, conversation.group.preview ?? ""]
            : []
      return values.some((value) =>
        value.toLocaleLowerCase().includes(normalizedQuery),
      )
    })
  }, [conversationName, normalizedQuery, scopedConversations])

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

  const selectedPool = listLoading ? allConversations : scopedConversations
  const selectedFromPool =
    selectedPool.find(
      (conversation) => conversation.id === selectedConversationId,
    ) ?? (selectedConversationId ? undefined : selectedPool[0])
  useEffect(() => {
    if (selectedFromPool) setSelectedConversationSnapshot(selectedFromPool)
  }, [selectedFromPool])
  useEffect(() => {
    if (
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
    listLoading,
    onSelectedConversationChange,
    scopedConversations,
    selectedConversationId,
  ])
  const selectedConversation =
    selectedConversationSnapshot?.id === selectedConversationId &&
    selectedPool.some(
      (conversation) => conversation.id === selectedConversationId,
    )
      ? selectedConversationSnapshot
      : selectedFromPool

  /** 选中一个会话。 */
  function selectConversation(conversationId: string) {
    onSelectedConversationChange(conversationId)

    if (isNarrowViewport) {
      setIsNarrowDetailOpen(true)
    }
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

  const pane = (
    <div className="flex min-h-0 flex-1 flex-col">
      <InboxPaneTop
        query={query}
        onQueryChange={setQuery}
        railCollapsed={railCollapsed}
        onRailToggle={() => setRailCollapsed((collapsed) => !collapsed)}
        searchDisabled={listLoading || allConversations.length === 0}
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
            onScopeChange={(nextScope) =>
              onQueryChange({ scope: nextScope })
            }
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
            conversations={visibleConversations}
            hasAnyConversation={scopedConversations.length > 0}
            loading={listLoading}
            selectedId={selectedConversationId}
            onSelect={selectConversation}
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
        {isNarrowViewport ? null : selectedConversation ? (
          <section className="min-h-0 flex-1">
            <ConversationMain
              conversation={selectedConversation}
              onSessionMoved={showMovedCustomerConversation}
              onConversationChanged={refreshConversationAfterMessage}
              onGroupLeft={showConversationAfterGroupLeft}
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

      {selectedConversation ? (
        <Sheet open={isNarrowDetailOpen} onOpenChange={setIsNarrowDetailOpen}>
          <SheetContent className="data-[side=right]:w-full p-0 sm:max-w-lg">
            <SheetHeader className="sr-only">
              <SheetTitle>
                {t("conversationTitle", {
                  name: conversationName(selectedConversation),
                })}
              </SheetTitle>
              <SheetDescription>{t("detailDescription")}</SheetDescription>
            </SheetHeader>
            <ConversationMain
              conversation={selectedConversation}
              onSessionMoved={showMovedCustomerConversation}
              onConversationChanged={refreshConversationAfterMessage}
              onGroupLeft={showConversationAfterGroupLeft}
              narrowViewport
            />
          </SheetContent>
        </Sheet>
      ) : null}

      <StartDirectConversationDialog
        open={directDialogOpen}
        currentIdentityID={identity.user.identityId}
        onOpenChange={setDirectDialogOpen}
        onStarted={showStartedConversation}
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
