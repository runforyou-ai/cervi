import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  ChatSubjectKind,
  listConversationMessages,
  type ConversationMessage,
  type ConversationMessageListData,
} from "@/api"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useUserTimeZone } from "@/contexts/user-preferences"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

/** 按稳定消息顺序合并并去重时间线。 */
function mergeMessages(
  earlier: ConversationMessage[],
  current: ConversationMessage[],
) {
  const messages = new Map<string, ConversationMessage>()
  for (const message of [...earlier, ...current]) {
    messages.set(message.id, message)
  }
  return [...messages.values()].sort((left, right) => {
    const timeDifference =
      Date.parse(left.originatedAt) - Date.parse(right.originatedAt)
    return timeDifference || left.id.localeCompare(right.id)
  })
}

/** 返回时间线滚动视口。 */
function timelineViewport(root: HTMLDivElement | null) {
  return root?.querySelector<HTMLElement>(
    '[data-slot="scroll-area-viewport"]',
  )
}

/** 按 Helmdesk 的 MM-DD HH:mm 格式显示用户时区中的消息时间。 */
function formatMessageTime(formatter: Intl.DateTimeFormat, date: Date) {
  const parts = Object.fromEntries(
    formatter
      .formatToParts(date)
      .filter((part) => part.type !== "literal")
      .map((part) => [part.type, part.value]),
  )
  return `${parts.month}-${parts.day} ${parts.hour}:${parts.minute}`
}

/** 客户会话只读消息时间线。 */
export function ConversationTimeline({
  conversationID,
}: {
  conversationID: string
}) {
  const { t, i18n } = useTranslation("inbox")
  const navigate = useNavigate()
  const timeZone = useUserTimeZone()
  const scrollRootRef = useRef<HTMLDivElement>(null)
  const aliveRef = useRef(true)
  const beforeRequestRef = useRef(0)
  const initialScrollRef = useRef(true)
  const previousScrollHeightRef = useRef<number | null>(null)
  const [timeline, setTimeline] =
    useState<ConversationMessageListData | null>(null)
  const [loadingEarlier, setLoadingEarlier] = useState(false)
  const [earlierError, setEarlierError] = useState(false)
  const { data, loading, error, refresh } = useResource(
    resourceKeys.conversationMessages(conversationID),
    (signal) => listConversationMessages(conversationID, undefined, signal),
  )
  const currentPage = timeline ?? data

  const dateFormatters = useMemo(() => {
    const locale = i18n.resolvedLanguage
    return {
      messageTime: new Intl.DateTimeFormat("en-US", {
        timeZone,
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        hourCycle: "h23",
      }),
      full: new Intl.DateTimeFormat(locale, {
        timeZone,
        dateStyle: "medium",
        timeStyle: "short",
      }),
    }
  }, [i18n.resolvedLanguage, timeZone])

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
      beforeRequestRef.current += 1
    }
  }, [])

  useLayoutEffect(() => {
    const viewport = timelineViewport(scrollRootRef.current)
    if (!viewport || !currentPage) return
    if (initialScrollRef.current) {
      initialScrollRef.current = false
      viewport.scrollTop = viewport.scrollHeight
      return
    }
    if (previousScrollHeightRef.current !== null) {
      viewport.scrollTop +=
        viewport.scrollHeight - previousScrollHeightRef.current
      previousScrollHeightRef.current = null
    }
  }, [currentPage])

  /** 加载并前插一页更早消息。 */
  async function loadEarlier() {
    const before = currentPage?.before
    if (!before || loadingEarlier) return
    const request = beforeRequestRef.current + 1
    beforeRequestRef.current = request
    const viewport = timelineViewport(scrollRootRef.current)
    previousScrollHeightRef.current = viewport?.scrollHeight ?? null
    setLoadingEarlier(true)
    setEarlierError(false)
    try {
      const earlierPage = await listConversationMessages(conversationID, {
        before,
        after: "",
      })
      if (!aliveRef.current || beforeRequestRef.current !== request) return
      setTimeline((current) => {
        const base = current ?? currentPage
        if (!base || base.before !== before) return current
        return {
          messages: mergeMessages(earlierPage.messages, base.messages),
          before: earlierPage.before,
          after: base.after,
        }
      })
    } catch (requestError) {
      if (!aliveRef.current || beforeRequestRef.current !== request) return
      previousScrollHeightRef.current = null
      if (recoverSession(requestError, navigate)) return
      setEarlierError(true)
      console.warn("加载更早会话消息失败", {
        conversationId: conversationID,
        error: requestError,
      })
    } finally {
      if (aliveRef.current && beforeRequestRef.current === request) {
        setLoadingEarlier(false)
      }
    }
  }

  if (loading && !currentPage) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center gap-2 bg-background text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("messagesLoading")}
      </div>
    )
  }

  if (error && !currentPage) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center bg-background p-6 text-center">
        <div>
          <p className="text-sm text-muted-foreground">
            {t("messagesLoadError")}
          </p>
          <Button
            className="mt-4"
            size="sm"
            variant="outline"
            onClick={() => void refresh()}
          >
            {t("messagesRetry")}
          </Button>
        </div>
      </div>
    )
  }

  if (!currentPage || currentPage.messages.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center bg-background p-6 text-sm text-muted-foreground">
        {t("messagesEmpty")}
      </div>
    )
  }

  return (
    <ScrollArea ref={scrollRootRef} className="min-h-0 flex-1 bg-background">
      <div className="flex w-full flex-col px-4 pb-3 md:px-6">
        {currentPage.before || earlierError ? (
          <div className="flex items-center justify-center py-2">
            {currentPage.before ? (
              <Button
                size="sm"
                variant="ghost"
                disabled={loadingEarlier}
                onClick={() => void loadEarlier()}
              >
                {loadingEarlier
                  ? t("messagesLoadingEarlier")
                  : t("messagesLoadEarlier")}
              </Button>
            ) : null}
            {earlierError ? (
              <span className="ml-2 text-xs text-destructive" role="status">
                {t("messagesLoadEarlierError")}
              </span>
            ) : null}
          </div>
        ) : null}

        <div className="grid gap-3">
          {currentPage.messages.map((message) => {
            const date = new Date(message.originatedAt)
            const incoming =
              !message.sender ||
              message.sender.kind === ChatSubjectKind.ChatSubjectKindContact
            const senderName =
              message.sender?.displayName?.trim() ||
              (message.sender?.kind === ChatSubjectKind.ChatSubjectKindContact
                ? t("anonymousVisitor")
                : t("unknownSender"))
            const senderInitial =
              Array.from(senderName)[0]?.toLocaleUpperCase() ?? "?"

            return (
              <article
                key={message.id}
                className={cn(
                  "flex items-start gap-2",
                  incoming ? "justify-start" : "justify-end",
                )}
                aria-label={`${senderName} ${dateFormatters.full.format(date)}`}
              >
                <div
                  className={cn(
                    "flex max-w-[75%] flex-col gap-1",
                    incoming ? "ml-10 items-start" : "mr-10 items-end",
                  )}
                >
                  <time
                    dateTime={message.originatedAt}
                    title={dateFormatters.full.format(date)}
                    className="text-[11px] text-muted-foreground/80"
                  >
                    {formatMessageTime(dateFormatters.messageTime, date)}
                  </time>
                  <div className="relative max-w-full">
                    <span
                      className={cn(
                        "absolute bottom-0 flex size-8 items-center justify-center rounded-full text-xs font-medium",
                        incoming
                          ? "right-full mr-2 border bg-background text-foreground"
                          : "left-full ml-2 bg-primary text-primary-foreground",
                      )}
                      title={senderName}
                      aria-hidden="true"
                    >
                      {senderInitial}
                    </span>
                    <div
                      className={cn(
                        "min-w-0 max-w-full rounded-2xl px-3 py-2 text-sm break-words whitespace-pre-wrap [overflow-wrap:anywhere]",
                        incoming
                          ? "rounded-bl-sm border bg-background text-foreground shadow-xs"
                          : "rounded-br-sm bg-primary text-primary-foreground",
                      )}
                    >
                      {message.body}
                    </div>
                  </div>
                </div>
              </article>
            )
          })}
        </div>
      </div>
    </ScrollArea>
  )
}
