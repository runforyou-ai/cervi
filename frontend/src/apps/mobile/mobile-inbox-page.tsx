/** 移动端统一会话摘要列表和内部单聊入口。 */
import { useEffect, useMemo, useRef, useState } from "react"
import type { TFunction } from "i18next"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  InboxScope,
  ConversationStatus,
  loadInbox,
  ServiceSessionStatus,
  type CustomerInboxConversationData,
  type DirectInboxConversationData,
  type InboxConversation,
  type GroupInboxConversationData,
} from "@/api"
import {
  MobileCustomerFilter,
  MobileInboxScopes,
  useMobileInboxQuery,
} from "@/apps/mobile/mobile-inbox-navigation"
import {
  MobilePageHeader,
  MobilePageState,
  MobileScrollArea,
} from "@/apps/mobile/mobile-page"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import { Button } from "@/components/ui/button"
import { LoadingIndicator } from "@/components/loading-indicator"
import { useUserTimeZone } from "@/contexts/user-preferences"
import { previousDayKey } from "@/features/inbox/calendar"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

type MobileInboxConversation =
  | CustomerInboxConversationData
  | DirectInboxConversationData
  | GroupInboxConversationData

/** 识别移动端展示的三种会话摘要。 */
function isMobileInboxConversation(
  conversation: InboxConversation,
): conversation is MobileInboxConversation {
  return (
    isCustomerInboxConversation(conversation) ||
    isDirectInboxConversation(conversation) ||
    isGroupInboxConversation(conversation)
  )
}

/** 返回客服处理状态的移动端文案。 */
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

/** 按移动端消息列表习惯格式化最近消息时间。 */
function useConversationTime() {
  const { t, i18n } = useTranslation("inbox")
  const timeZone = useUserTimeZone()

  return useMemo(() => {
    const locale = i18n.resolvedLanguage
    const relative = new Intl.RelativeTimeFormat(locale, { numeric: "always" })
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
      if (elapsedMs < 60_000) return t("justNow")
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
      if (day.slice(0, 4) === dayKey.format(now).slice(0, 4)) {
        return monthDay.format(date)
      }
      return fullDate.format(date)
    }
  }, [i18n.resolvedLanguage, t, timeZone])
}

/** 每分钟刷新一次相对时间。 */
function useMinuteTick() {
  const [, setTick] = useState(0)
  useEffect(() => {
    const timer = window.setInterval(() => setTick((tick) => tick + 1), 60_000)
    return () => window.clearInterval(timer)
  }, [])
}

/** 渲染会话摘要，单聊可进入详情，客户和群聊明确标注只读范围。 */
function MobileConversationRow({
  conversation,
  onOpenDirect,
}: {
  conversation: MobileInboxConversation
  onOpenDirect: (conversation: DirectInboxConversationData) => void
}) {
  const { t } = useTranslation("inbox")
  const { t: tMobile } = useTranslation("mobile")
  const formatTime = useConversationTime()
  const customerConversation = isCustomerInboxConversation(conversation)
    ? conversation
    : null
  const directConversation = isDirectInboxConversation(conversation)
    ? conversation
    : null
  const groupConversation = isGroupInboxConversation(conversation)
    ? conversation
    : null
  const name = customerConversation
    ? (customerConversation.customer.contactName ?? t("anonymousVisitor"))
    : (
        directConversation?.direct.peerName ?? groupConversation?.group.title
      )?.trim() || t("unknownSender")
  const summary =
    customerConversation?.customer ??
    directConversation?.direct ??
    groupConversation?.group
  const agentRunLabel = agentRunStatusLabel(
    directConversation?.direct.agentRunStatus ?? null,
    t,
  )
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
      : (summary.preview ??
        (groupConversation && conversation.lastMessageId
          ? t("groupSystemUpdated")
          : t("messagesEmpty")))
  const formattedTime = formatTime(summary.lastMessageAt)

  const content = (
    <>
      <ConversationAvatar conversation={conversation} />
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
        <p
          title={preview}
          className="mt-0.5 w-full min-w-0 truncate text-sm text-muted-foreground"
        >
          {preview}
        </p>
        {!directConversation ? (
          <p className="mt-1 text-xs text-muted-foreground">
            {tMobile(
              groupConversation
                ? "inbox.groupUnavailable"
                : "inbox.customerSummaryOnly",
            )}
          </p>
        ) : null}
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
    <li className="border-b last:border-b-0">
      {directConversation ? (
        <button
          type="button"
          className="flex w-full min-w-0 gap-3 px-4 py-3 text-left outline-none transition-colors active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
          aria-label={name}
          onClick={() => onOpenDirect(directConversation)}
        >
          {content}
        </button>
      ) : (
        <div className="flex min-w-0 gap-3 px-4 py-3">{content}</div>
      )}
    </li>
  )
}

/** 加载当前范围的真实会话摘要并恢复列表浏览位置。 */
export function MobileInboxPage() {
  const { t } = useTranslation("mobile")
  const navigate = useNavigate()
  const { query, changeQuery } = useMobileInboxQuery()
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const previousPollingActiveRef = useRef(pollingActive)
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.inbox(query),
    () => loadInbox(query),
    {
      staleTime: 0,
      refetchInterval: pollingActive ? memberChatPollingInterval : false,
      refetchOnWindowFocus: false,
    },
  )
  useMinuteTick()
  const conversations =
    data?.conversations.filter(isMobileInboxConversation) ?? []
  const scrollKey = `inbox:${query.scope}:${query.customerView}:${query.assigneeIdentityId}`

  useEffect(() => {
    if (pollingActive && !previousPollingActiveRef.current && data)
      void refresh()
    previousPollingActiveRef.current = pollingActive
  }, [data, pollingActive, refresh])

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t("inbox.title")}
        actions={
          <Button
            variant="ghost"
            className={cn("min-h-11", error && "text-warning")}
            aria-label={error ? t("inbox.refreshError") : t("inbox.refresh")}
            disabled={loading || refreshing}
            onClick={() => void refresh()}
          >
            {refreshing
              ? t("inbox.refreshing")
              : error
                ? t("inbox.refreshFailed")
                : t("inbox.refresh")}
          </Button>
        }
      />
      <MobileInboxScopes scope={query.scope} onChange={changeQuery} />
      {query.scope === InboxScope.InboxScopeCustomer ? (
        <div className="flex h-11 shrink-0 items-center border-b">
          <MobileCustomerFilter query={query} onChange={changeQuery} />
        </div>
      ) : null}
      <MobileScrollArea
        storageKey={scrollKey}
        ready={Boolean(data)}
        className="flex flex-col"
      >
        {loading && !data ? (
          <LoadingIndicator className="min-h-64 flex-1 justify-center">
            {t("loading")}
          </LoadingIndicator>
        ) : null}
        {!loading && !data ? (
          <MobilePageState
            title={t("inbox.loadError")}
            onRetry={() => void refresh()}
          />
        ) : null}
        {data && conversations.length === 0 ? (
          <MobilePageState
            title={t("inbox.emptyTitle")}
            description={t("inbox.emptyDescription")}
          />
        ) : null}
        {data && conversations.length > 0 ? (
          <ul>
            {conversations.map((conversation) => (
              <MobileConversationRow
                key={conversation.id}
                conversation={conversation}
                onOpenDirect={(direct) => {
                  navigate(`/inbox/direct/${direct.id}`, {
                    state: { conversation: direct, mobileBack: true },
                  })
                }}
              />
            ))}
          </ul>
        ) : null}
      </MobileScrollArea>
    </section>
  )
}
