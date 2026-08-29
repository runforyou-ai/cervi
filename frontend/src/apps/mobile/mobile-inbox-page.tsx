/** 移动端客户会话摘要列表。 */
import { useEffect, useMemo, useState } from "react"
import type { TFunction } from "i18next"
import {
  GlobeIcon,
  LoaderCircleIcon,
  MessageCircleIcon,
  MessagesSquareIcon,
  RefreshCwIcon,
  SendIcon,
  UserRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  ChannelType,
  loadInbox,
  ServiceSessionStatus,
  type InboxConversation,
} from "@/api"
import { Button } from "@/components/ui/button"
import { useUserTimeZone } from "@/contexts/user-preferences"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

const sourceBadges: Partial<
  Record<ChannelType, { icon: typeof GlobeIcon; className: string }>
> = {
  [ChannelType.ChannelTypeWebsite]: {
    icon: GlobeIcon,
    className: "bg-badge-website",
  },
  [ChannelType.ChannelTypeTelegram]: {
    icon: SendIcon,
    className: "bg-badge-telegram",
  },
  [ChannelType.ChannelTypeWeChatOfficialAccount]: {
    icon: MessageCircleIcon,
    className: "bg-badge-wechat",
  },
}

/** 返回客服处理状态的移动端文案。 */
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

/** 返回客服处理状态的移动端颜色。 */
function sessionStatusClass(status: ServiceSessionStatus) {
  switch (status) {
    case ServiceSessionStatus.ServiceSessionStatusWaiting:
      return "bg-warning/15 text-warning"
    case ServiceSessionStatus.ServiceSessionStatusActive:
      return "bg-primary/10 text-primary"
    default:
      return "bg-muted text-muted-foreground"
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

    return (value: string) => {
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
      if (day === dayKey.format(new Date(now.getTime() - 86_400_000))) {
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

/** 展示移动端会话头像和来源角标。 */
function MobileConversationAvatar({
  conversation,
}: {
  conversation: InboxConversation
}) {
  const badge = sourceBadges[conversation.channelType]

  return (
    <div className="relative shrink-0">
      <div className="flex size-11 items-center justify-center rounded-xl bg-muted text-sm font-medium text-muted-foreground">
        {conversation.contactName ? (
          conversation.contactName.slice(0, 1).toLocaleUpperCase()
        ) : (
          <UserRoundIcon className="size-4.5" />
        )}
      </div>
      {badge ? (
        <span
          aria-hidden="true"
          className={cn(
            "absolute -right-0.5 -bottom-0.5 flex size-4 items-center justify-center rounded-full border-2 border-background text-white",
            badge.className,
          )}
        >
          <badge.icon className="size-2" />
        </span>
      ) : null}
    </div>
  )
}

/** 渲染一条不可进入详情的客户会话摘要。 */
function MobileConversationRow({
  conversation,
}: {
  conversation: InboxConversation
}) {
  const { t } = useTranslation("inbox")
  const formatTime = useConversationTime()
  const name = conversation.contactName ?? t("anonymousVisitor")

  return (
    <li className="flex gap-3 border-b px-4 py-3 last:border-b-0">
      <MobileConversationAvatar conversation={conversation} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="truncate text-[15px] font-medium">{name}</p>
          <time className="ml-auto shrink-0 text-xs text-muted-foreground">
            {formatTime(conversation.lastMessageAt)}
          </time>
        </div>
        <p className="mt-0.5 truncate text-sm text-muted-foreground">
          {conversation.preview}
        </p>
        <div className="mt-1.5 flex min-w-0 items-center gap-2">
          <span className="truncate text-xs text-muted-foreground">
            {conversation.channelName}
          </span>
          <span
            className={cn(
              "ml-auto shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium",
              sessionStatusClass(conversation.serviceSessionStatus),
            )}
          >
            {sessionStatusLabel(conversation.serviceSessionStatus, t)}
          </span>
        </div>
      </div>
    </li>
  )
}

/** 加载并显示移动端客户会话摘要。 */
export function MobileInboxPage() {
  const { t } = useTranslation("mobile")
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.inbox(),
    () => loadInbox(),
    { staleTime: 0 },
  )
  useMinuteTick()

  useEffect(() => {
    /** 应用回到前台时刷新会话摘要。 */
    function refreshWhenVisible() {
      if (document.visibilityState === "visible") void refresh()
    }
    document.addEventListener("visibilitychange", refreshWhenVisible)
    return () => {
      document.removeEventListener("visibilitychange", refreshWhenVisible)
    }
  }, [refresh])

  return (
    <section className="flex h-full min-h-0 flex-col">
      <header className="flex h-14 shrink-0 items-center border-b px-4">
        <h1 className="text-lg font-semibold tracking-tight">
          {t("inbox.title")}
        </h1>
        <Button
          className="ml-auto"
          variant="ghost"
          size="icon-lg"
          disabled={loading || refreshing}
          aria-label={t("inbox.refresh")}
          onClick={() => void refresh()}
        >
          <RefreshCwIcon className={cn(refreshing && "animate-spin")} />
        </Button>
      </header>

      {loading ? (
        <div className="flex min-h-0 flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
          <LoaderCircleIcon className="size-4 animate-spin" />
          {t("loading")}
        </div>
      ) : null}

      {!loading && !data ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 text-center">
          <div>
            <p className="text-sm text-muted-foreground">
              {t("inbox.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refresh()}
            >
              <RefreshCwIcon />
              {t("retry")}
            </Button>
          </div>
        </div>
      ) : null}

      {data ? (
        <>
          {error ? (
            <button
              type="button"
              className="min-h-11 w-full shrink-0 border-b bg-warning/10 px-4 py-2 text-center text-xs text-warning"
              onClick={() => void refresh()}
            >
              {t("inbox.refreshError")}
            </button>
          ) : null}
          {data.conversations.length === 0 ? (
            <div className="flex min-h-0 flex-1 items-center justify-center px-6 text-center">
              <div className="max-w-xs">
                <div className="mx-auto mb-4 flex size-11 items-center justify-center rounded-xl border shadow-sm">
                  <MessagesSquareIcon className="size-5 text-muted-foreground" />
                </div>
                <h2 className="text-base font-semibold">
                  {t("inbox.emptyTitle")}
                </h2>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t("inbox.emptyDescription")}
                </p>
              </div>
            </div>
          ) : (
            <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
              <ul>
                {data.conversations.map((conversation) => (
                  <MobileConversationRow
                    key={conversation.id}
                    conversation={conversation}
                  />
                ))}
              </ul>
            </div>
          )}
        </>
      ) : null}
    </section>
  )
}
