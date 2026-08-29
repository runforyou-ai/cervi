/** 消息页中栏（范围纵栏 + 会话列表）和会话主区。 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import type { TFunction } from "i18next"
import {
  ChevronDownIcon,
  HeadsetIcon,
  MessagesSquareIcon,
  PanelLeftIcon,
  PlusIcon,
  SearchIcon,
  XIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import { ServiceSessionStatus, type InboxConversation } from "@/api"
import { PageSplit } from "@/components/page-split"
import { StatusBadge } from "@/components/status-badge"
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
import { previousDayKey } from "@/features/inbox/calendar"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import { ConversationContextPane } from "@/features/inbox/conversation-context-pane"
import {
  ConversationAvatar,
  ConversationHeader,
} from "@/features/inbox/conversation-header"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import {
  useIsNarrowViewport,
  useIsWideViewport,
} from "@/hooks/use-narrow-viewport"
import { cn } from "@/lib/utils"

/** 消息范围；后续阶段出现内部会话等来源后按能力扩展。 */
type InboxScope = "all" | "customer"

/** 客户范围的队列子筛选；与 ServiceSession「批次状态 + 负责人」同构。 */
type CustomerQueueFilter = "queue" | "mine" | "ai" | "colleague" | "closed"

const scopes = [
  { id: "all", labelKey: "scopeAll", icon: MessagesSquareIcon },
  { id: "customer", labelKey: "scopeCustomer", icon: HeadsetIcon },
] as const

/** 客服处理状态文案。 */
function sessionStatusLabel(
  status: ServiceSessionStatus,
  t: TFunction<"inbox">,
) {
  switch (status) {
    case ServiceSessionStatus.ServiceSessionStatusWaiting:
      return t("sessionStatus.waiting")
    case ServiceSessionStatus.ServiceSessionStatusActive:
      return t("sessionStatus.active")
    case ServiceSessionStatus.ServiceSessionStatusPending:
      return t("sessionStatus.pending")
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
    (conversation: InboxConversation) =>
      conversation.contactName?.trim() || t("anonymousVisitor"),
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

    return (value: string) => {
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
}: {
  query: string
  onQueryChange: (query: string) => void
  railCollapsed: boolean
  onRailToggle: () => void
  searchDisabled: boolean
}) {
  const { t } = useTranslation("inbox")
  const { t: tCommon } = useTranslation("common")
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
    <div className="flex h-14 shrink-0 items-center gap-2 border-b border-border/60 px-3">
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
          <DropdownMenuItem disabled className="gap-2">
            <span className="min-w-0 flex-1 truncate">
              {t("newDirectConversation")}
            </span>
            <StatusBadge variant="muted">{tCommon("comingSoon")}</StatusBadge>
          </DropdownMenuItem>
          <DropdownMenuItem disabled className="gap-2">
            <span className="min-w-0 flex-1 truncate">
              {t("newGroupConversation")}
            </span>
            <StatusBadge variant="muted">{tCommon("comingSoon")}</StatusBadge>
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

/** 客户范围的队列子筛选：高频三段下划线页签平铺，低频项收进「更多」下拉。当前仅样式交互，尚未过滤列表。 */
function InboxCustomerQueueFilter({
  filter,
  onFilterChange,
  waitingCount,
}: {
  filter: CustomerQueueFilter
  onFilterChange: (filter: CustomerQueueFilter) => void
  waitingCount: number
}) {
  const { t } = useTranslation("inbox")

  const primarySegments = [
    { id: "queue", label: t("queueFilterQueue") },
    { id: "mine", label: t("queueFilterMine") },
    { id: "ai", label: t("queueFilterAi") },
  ] as const

  const moreSegments = [
    { id: "colleague", label: t("queueFilterColleague") },
    { id: "closed", label: t("queueFilterClosed") },
  ] as const

  const activeMoreSegment = moreSegments.find(
    (segment) => segment.id === filter,
  )

  return (
    <div
      role="tablist"
      aria-label={t("queueFilterLabel")}
      className="flex shrink-0 items-stretch border-b border-border/60 pt-2"
    >
      <div className="flex min-w-0 flex-1 items-stretch justify-between pl-2">
        {primarySegments.map((segment) => {
          const active = filter === segment.id
          return (
            <button
              key={segment.id}
              type="button"
              role="tab"
              aria-selected={active}
              className={cn(
                "relative h-8 min-w-0 rounded-md px-1 text-center text-sm font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring",
                active
                  ? "text-foreground"
                  : "text-muted-foreground hover:text-foreground",
              )}
              onClick={() => onFilterChange(segment.id)}
            >
              <span className="block truncate">{segment.label}</span>
              {segment.id === "queue" && waitingCount > 0 ? (
                <span
                  className={cn(
                    "pointer-events-none absolute -top-1 -right-1 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] leading-none tabular-nums shadow-sm ring-1 ring-background",
                    active
                      ? "bg-primary text-primary-foreground"
                      : "bg-secondary text-secondary-foreground",
                  )}
                >
                  {waitingCount > 99 ? "99+" : waitingCount}
                </span>
              ) : null}
              {active ? (
                <span
                  aria-hidden="true"
                  className="absolute right-1 -bottom-px left-1 h-0.5 rounded bg-primary"
                />
              ) : null}
            </button>
          )
        })}
      </div>
      <span
        aria-hidden="true"
        className="mx-1.5 my-2 w-px self-stretch bg-border/60"
      />
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            role="tab"
            aria-selected={Boolean(activeMoreSegment)}
            title={activeMoreSegment?.label ?? t("queueFilterMore")}
            className={cn(
              "relative h-8 w-24 shrink-0 rounded-md text-center text-sm font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring",
              activeMoreSegment
                ? "text-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            <span className="flex min-w-0 items-center justify-center gap-0.5">
              <span className="truncate">
                {activeMoreSegment?.label ?? t("queueFilterMore")}
              </span>
              <ChevronDownIcon className="size-3.5 opacity-70" />
            </span>
            {activeMoreSegment ? (
              <span
                aria-hidden="true"
                className="absolute right-0.5 -bottom-px left-0.5 h-0.5 rounded bg-primary"
              />
            ) : null}
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-28">
          {moreSegments.map((segment) => (
            <DropdownMenuItem
              key={segment.id}
              className={cn(
                filter === segment.id && "bg-muted text-foreground",
              )}
              onSelect={() => onFilterChange(segment.id)}
            >
              {segment.label}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}

/** 会话列表。 */
function InboxConversationList({
  conversations,
  hasAnyConversation,
  selectedId,
  onSelect,
}: {
  conversations: InboxConversation[]
  hasAnyConversation: boolean
  selectedId?: string
  onSelect: (conversationId: string) => void
}) {
  const { t } = useTranslation("inbox")
  const conversationName = useConversationName()
  const formatTime = useConversationTime()
  useMinuteTick()

  if (conversations.length === 0) {
    return hasAnyConversation ? (
      <p className="px-6 py-12 text-center text-sm text-muted-foreground">
        {t("searchEmpty")}
      </p>
    ) : null
  }

  return (
    <ScrollArea className="min-h-0 flex-1">
      <div className="grid pb-1.5">
        {conversations.map((conversation) => {
          const name = conversationName(conversation)
          return (
            <button
              key={conversation.id}
              type="button"
              aria-pressed={selectedId === conversation.id}
              aria-label={name}
              className={cn(
                "flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors",
                selectedId === conversation.id
                  ? "bg-accent text-accent-foreground"
                  : "hover:bg-muted",
              )}
              onClick={() => onSelect(conversation.id)}
            >
              <ConversationAvatar conversation={conversation} />
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-2">
                  <span className="truncate text-sm font-medium">
                    {name}
                  </span>
                  <span
                    className={cn(
                      "ml-auto shrink-0 text-xs text-muted-foreground",
                      selectedId === conversation.id &&
                        "text-accent-foreground/75",
                    )}
                  >
                    {formatTime(conversation.lastMessageAt)}
                  </span>
                </span>
                <span
                  className={cn(
                    "mt-0.5 block truncate text-xs text-muted-foreground",
                    selectedId === conversation.id &&
                      "text-accent-foreground/75",
                  )}
                >
                  {conversation.preview}
                </span>
              </span>
            </button>
          )
        })}
      </div>
    </ScrollArea>
  )
}

/** 组合当前 Conversation 的头部、时间线、回复区和上下文栏。 */
function ConversationMain({
  conversation,
  narrowViewport = false,
}: {
  conversation: InboxConversation
  narrowViewport?: boolean
}) {
  const { t } = useTranslation("inbox")
  const isWideViewport = useIsWideViewport()
  const conversationName = useConversationName()
  const [contextSheetOpen, setContextSheetOpen] = useState(false)
  const [contextCollapsed, setContextCollapsed] = useState(false)
  const contactName = conversationName(conversation)
  const sessionStatus = sessionStatusLabel(conversation.serviceSessionStatus, t)
  const desktopContextVisible = isWideViewport && !contextCollapsed
  const contextVisible = isWideViewport
    ? desktopContextVisible
    : contextSheetOpen

  useEffect(() => {
    if (isWideViewport) {
      setContextSheetOpen(false)
    }
  }, [isWideViewport])

  /** 切换当前视口使用的上下文栏。 */
  function toggleContext() {
    if (isWideViewport) {
      setContextCollapsed((collapsed) => !collapsed)
      return
    }
    setContextSheetOpen((open) => !open)
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <ConversationHeader
        conversation={conversation}
        contactName={contactName}
        sessionStatus={sessionStatus}
        contextVisible={contextVisible}
        narrowViewport={narrowViewport}
        onContextToggle={toggleContext}
      />
      <div className="flex min-h-0 flex-1">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <ConversationTimeline
            key={conversation.id}
            conversationID={conversation.id}
          />
          <ConversationComposer
            key={conversation.id}
            conversationID={conversation.id}
          />
        </div>
        <ConversationContextPane
          conversation={conversation}
          contactName={contactName}
          sessionStatus={sessionStatus}
          desktopVisible={desktopContextVisible}
          sheetOpen={!isWideViewport && contextSheetOpen}
          onDesktopCollapse={() => setContextCollapsed(true)}
          onSheetOpenChange={setContextSheetOpen}
        />
      </div>
    </div>
  )
}

/** 消息页中栏和当前会话。 */
export function InboxPage({
  conversations,
}: {
  conversations: InboxConversation[]
}) {
  const { t } = useTranslation("inbox")
  const isNarrowViewport = useIsNarrowViewport()
  const [scope, setScope] = useState<InboxScope>("all")
  const [railCollapsed, setRailCollapsed] = useState(false)
  const [queueFilter, setQueueFilter] = useState<CustomerQueueFilter>("mine")
  const [query, setQuery] = useState("")
  const [selectedId, setSelectedId] = useState(() => conversations[0]?.id ?? "")
  const [isNarrowDetailOpen, setIsNarrowDetailOpen] = useState(false)
  const conversationName = useConversationName()

  /* 首期只有客户会话，两个范围返回同一列表；出现内部会话后按范围过滤。 */
  const scopedConversations = conversations

  const normalizedQuery = query.trim().toLocaleLowerCase()
  const visibleConversations = useMemo(() => {
    if (!normalizedQuery) {
      return scopedConversations
    }
    return scopedConversations.filter((conversation) =>
      [
        conversationName(conversation),
        conversation.title,
        conversation.preview,
        conversation.channelName,
      ].some((value) => value.toLocaleLowerCase().includes(normalizedQuery)),
    )
  }, [conversationName, normalizedQuery, scopedConversations])

  useEffect(() => {
    if (!isNarrowViewport) {
      setIsNarrowDetailOpen(false)
    }
  }, [isNarrowViewport])

  useEffect(() => {
    if (!conversations.some((conversation) => conversation.id === selectedId)) {
      setSelectedId(conversations[0]?.id ?? "")
    }
  }, [conversations, selectedId])

  const selectedConversation =
    conversations.find((conversation) => conversation.id === selectedId) ??
    conversations[0]

  /** 选中一个会话。 */
  function selectConversation(conversationId: string) {
    setSelectedId(conversationId)

    if (isNarrowViewport) {
      setIsNarrowDetailOpen(true)
    }
  }

  const pane = (
    <div className="flex min-h-0 flex-1 flex-col">
      <InboxPaneTop
        query={query}
        onQueryChange={setQuery}
        railCollapsed={railCollapsed}
        onRailToggle={() => setRailCollapsed((collapsed) => !collapsed)}
        searchDisabled={conversations.length === 0}
      />
      <div className="flex min-h-0 flex-1">
        {railCollapsed ? null : (
          <InboxScopeRail scope={scope} onScopeChange={setScope} />
        )}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          {scope === "customer" ? (
            <InboxCustomerQueueFilter
              filter={queueFilter}
              onFilterChange={setQueueFilter}
              waitingCount={
                conversations.filter(
                  (conversation) =>
                    conversation.serviceSessionStatus ===
                    ServiceSessionStatus.ServiceSessionStatusWaiting,
                ).length
              }
            />
          ) : null}
          <InboxConversationList
            conversations={visibleConversations}
            hasAnyConversation={conversations.length > 0}
            selectedId={selectedId}
            onSelect={selectConversation}
          />
        </div>
      </div>
    </div>
  )

  if (conversations.length === 0) {
    return (
      <PageSplit
        paneWidth={railCollapsed ? "inboxCollapsed" : "inbox"}
        className="bg-background"
        paneClassName="transition-[width]"
        pane={pane}
        mainClassName="cervi-inbox-empty-main items-center justify-center p-6"
      >
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
      </PageSplit>
    )
  }

  return (
    <>
      <PageSplit
        paneWidth={railCollapsed ? "inboxCollapsed" : "inbox"}
        paneOnNarrow="fill"
        className="bg-background"
        paneClassName="transition-[width]"
        pane={pane}
      >
        {isNarrowViewport ? null : (
          <section className="min-h-0 flex-1">
            <ConversationMain conversation={selectedConversation} />
          </section>
        )}
      </PageSplit>

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
            narrowViewport
          />
        </SheetContent>
      </Sheet>
    </>
  )
}
