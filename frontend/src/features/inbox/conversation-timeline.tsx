/** 读取并展示当前客户 Conversation 的文本消息。 */
import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

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
import { cn } from "@/lib/utils"

type TimelineState = ConversationMessageListData

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

/** 客户会话只读消息时间线。 */
export function ConversationTimeline({
  conversationID,
}: {
  conversationID: string
}) {
  const { t, i18n } = useTranslation("inbox")
  const timeZone = useUserTimeZone()
  const scrollRootRef = useRef<HTMLDivElement>(null)
  const initializedRef = useRef(false)
  const aliveRef = useRef(true)
  const beforeRequestRef = useRef(0)
  const initialScrollRef = useRef(false)
  const previousScrollHeightRef = useRef<number | null>(null)
  const [timeline, setTimeline] = useState<TimelineState | null>(null)
  const [loadingEarlier, setLoadingEarlier] = useState(false)
  const [earlierError, setEarlierError] = useState(false)
  const { data, loading, error, refresh } = useResource(
    resourceKeys.conversationMessages(conversationID),
    (signal) => listConversationMessages(conversationID, undefined, signal),
  )

  const dateFormatters = useMemo(() => {
    const locale = i18n.resolvedLanguage
    return {
      dayKey: new Intl.DateTimeFormat("en-CA", {
        timeZone,
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
      }),
      day: new Intl.DateTimeFormat(locale, {
        timeZone,
        year: "numeric",
        month: "short",
        day: "numeric",
      }),
      time: new Intl.DateTimeFormat(locale, {
        timeZone,
        hour: "2-digit",
        minute: "2-digit",
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

  useEffect(() => {
    if (!data || initializedRef.current) return
    initializedRef.current = true
    initialScrollRef.current = true
    setTimeline(data)
  }, [data])

  useLayoutEffect(() => {
    const viewport = timelineViewport(scrollRootRef.current)
    if (!viewport || !timeline) return
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
  }, [timeline])

  /** 加载并前插一页更早消息。 */
  async function loadEarlier() {
    const before = timeline?.before
    if (!before || loadingEarlier) return
    const request = beforeRequestRef.current + 1
    beforeRequestRef.current = request
    const viewport = timelineViewport(scrollRootRef.current)
    previousScrollHeightRef.current = viewport?.scrollHeight ?? null
    setLoadingEarlier(true)
    setEarlierError(false)
    try {
      const page = await listConversationMessages(conversationID, {
        before,
        after: "",
      })
      if (!aliveRef.current || beforeRequestRef.current !== request) return
      setTimeline((current) => {
        if (!current || current.before !== before) return current
        return {
          messages: mergeMessages(page.messages, current.messages),
          before: page.before,
          after: current.after,
        }
      })
    } catch (error) {
      if (!aliveRef.current || beforeRequestRef.current !== request) return
      previousScrollHeightRef.current = null
      setEarlierError(true)
      console.warn("加载更早会话消息失败", {
        conversationId: conversationID,
        error,
      })
    } finally {
      if (aliveRef.current && beforeRequestRef.current === request) {
        setLoadingEarlier(false)
      }
    }
  }

  if (loading && !timeline) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center gap-2 bg-muted/20 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("messagesLoading")}
      </div>
    )
  }

  if (error && !timeline) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center bg-muted/20 p-6 text-center">
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

  if (!timeline || timeline.messages.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center bg-muted/20 p-6 text-sm text-muted-foreground">
        {t("messagesEmpty")}
      </div>
    )
  }

  const today = dateFormatters.dayKey.format(new Date())
  const yesterday = dateFormatters.dayKey.format(
    new Date(Date.now() - 86_400_000),
  )
  let previousDay = ""

  return (
    <ScrollArea ref={scrollRootRef} className="min-h-0 flex-1 bg-muted/20">
      <div className="mx-auto flex w-full max-w-4xl flex-col px-4 py-5 md:px-6">
        <div className="mb-5 flex min-h-7 items-center justify-center">
          {timeline.before ? (
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

        <div className="grid gap-3">
          {timeline.messages.map((message) => {
            const date = new Date(message.originatedAt)
            const day = dateFormatters.dayKey.format(date)
            const showDay = day !== previousDay
            previousDay = day
            const incoming =
              message.sender?.kind === ChatSubjectKind.ChatSubjectKindContact
            const senderName =
              message.sender?.displayName ??
              (incoming ? t("anonymousVisitor") : t("unknownSender"))
            const dayLabel =
              day === today
                ? t("messageToday")
                : day === yesterday
                  ? t("messageYesterday")
                  : dateFormatters.day.format(date)

            return (
              <div key={message.id}>
                {showDay ? (
                  <div className="my-3 flex items-center gap-3 text-xs text-muted-foreground">
                    <span className="h-px flex-1 bg-border/70" />
                    <time dateTime={message.originatedAt}>{dayLabel}</time>
                    <span className="h-px flex-1 bg-border/70" />
                  </div>
                ) : null}
                <article
                  className={cn(
                    "flex flex-col",
                    incoming ? "items-start" : "items-end",
                  )}
                >
                  <span className="mb-1 px-1 text-xs text-muted-foreground">
                    {senderName}
                  </span>
                  <div
                    className={cn(
                      "max-w-[min(82%,36rem)] rounded-2xl px-3.5 py-2.5 text-sm break-words whitespace-pre-wrap shadow-xs",
                      incoming
                        ? "rounded-bl-md border bg-background text-foreground"
                        : "rounded-br-md bg-primary text-primary-foreground",
                    )}
                  >
                    {message.body}
                  </div>
                  <time
                    dateTime={message.originatedAt}
                    className="mt-1 px-1 text-[11px] text-muted-foreground"
                  >
                    {dateFormatters.time.format(date)}
                  </time>
                </article>
              </div>
            )
          })}
        </div>
      </div>
    </ScrollArea>
  )
}
