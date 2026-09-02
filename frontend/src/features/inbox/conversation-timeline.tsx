/** 客服、单聊与群聊共用的成员消息时间线。 */
import {
  Fragment,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ChatSubjectKind,
  ConversationSystemEventType,
  ConversationType,
  MessageType,
  ServiceSessionStatus,
  listConversationMessages,
  type ConversationMessageData,
  type ConversationMessageListData,
  type ConversationMessageReference,
  type ConversationSystemEvent,
  type ConversationSystemEventParticipant,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useUserTimeZone } from "@/contexts/user-preferences"
import { previousDayKey } from "@/features/inbox/calendar"
import { mentionTokenPattern } from "@/features/inbox/mention-token"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import type {
  OutgoingConversationDraft,
  OutgoingConversationMessage,
} from "@/features/inbox/use-outgoing-conversation-messages"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

/** 按稳定消息顺序合并并去重时间线。 */
function mergeMessages(
  earlier: ConversationMessageData[],
  current: ConversationMessageData[],
) {
  const messages = new Map<string, ConversationMessageData>()
  for (const message of [...earlier, ...current]) {
    messages.set(message.id, message)
  }
  return [...messages.values()].sort((left, right) => {
    const timeDifference =
      Date.parse(left.originatedAt) - Date.parse(right.originatedAt)
    return timeDifference || left.id.localeCompare(right.id)
  })
}

type TimelineMessage = Pick<
  ConversationMessageData,
  | "id"
  | "type"
  | "body"
  | "originatedAt"
  | "sender"
  | "sessionStart"
  | "systemEvent"
  | "replyTo"
  | "mentions"
> & {
  clientMessageID: string | null
  mentionSubjectIDs: string[]
  local: boolean
  deliveryStatus: "sending" | "failed" | null
}

const timelineBottomThreshold = 48
const timelineGroupInterval = 5 * 60 * 1000

/** 判断时间线是否已经接近底部。 */
function timelineNearBottom(viewport: HTMLElement) {
  return (
    viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight <=
    timelineBottomThreshold
  )
}

/** 返回视觉分组使用的稳定发送者标识。 */
function timelineSenderKey(
  message: TimelineMessage,
  currentIdentityID: string,
) {
  if (message.local) {
    return `${ChatSubjectKind.ChatSubjectKindOrganizationIdentity}:${currentIdentityID}`
  }
  if (!message.sender) return `unknown:${message.id}`
  return `${message.sender.kind}:${message.sender.sourceId}`
}

/** 合并服务端消息和当前页面的即时发送状态。 */
function mergeTimelineMessages(
  current: ConversationMessageData[],
  outgoing: OutgoingConversationMessage[],
) {
  const saved = outgoing.flatMap((message) =>
    message.saved ? [message.saved] : [],
  )
  const messages: TimelineMessage[] = mergeMessages(current, saved).map(
    (message) => ({
      ...message,
      clientMessageID: null,
      mentionSubjectIDs: [],
      local: false,
      deliveryStatus: null,
    }),
  )
  for (const message of outgoing) {
    if (message.saved) continue
    messages.push({
      id: `local:${message.clientMessageID}`,
      type: MessageType.MessageTypeText,
      body: message.body,
      originatedAt: message.originatedAt,
      sender: null,
      sessionStart: null,
      systemEvent: null,
      replyTo: message.replyTo,
      mentions: [],
      clientMessageID: message.clientMessageID,
      mentionSubjectIDs: message.mentionSubjectIDs,
      local: true,
      deliveryStatus:
        message.status === "failed"
          ? "failed"
          : message.showSending
            ? "sending"
            : null,
    })
  }
  return messages.sort((left, right) => {
    const timeDifference =
      Date.parse(left.originatedAt) - Date.parse(right.originatedAt)
    return timeDifference || left.id.localeCompare(right.id)
  })
}

/** 在消息正文中强调服务端确认的结构化提醒。 */
function renderMessageBody(message: TimelineMessage) {
  const names = message.mentions
    .map((mention) => mention.displayName?.trim() ?? "")
    .filter((name, index, values) => name && values.indexOf(name) === index)
    .sort((left, right) => right.length - left.length)
  if (names.length === 0) return message.body
  const mentioned = new Set(names.map((name) => `@${name}`))
  const parts = message.body.split(
    new RegExp(`(${mentionTokenPattern(names)})`, "gu"),
  )
  return parts.map((part, index) =>
    mentioned.has(part) ? (
      <span key={`${part}:${index}`} className="font-semibold underline">
        {part}
      </span>
    ) : (
      part
    ),
  )
}

/** 返回时间线滚动视口。 */
function timelineViewport(root: HTMLDivElement | null) {
  return root?.querySelector<HTMLElement>(
    '[data-slot="scroll-area-viewport"]',
  )
}

/** 按 MM-DD HH:mm 格式显示用户时区中的消息时间。 */
function formatMessageTime(formatter: Intl.DateTimeFormat, date: Date) {
  const parts = Object.fromEntries(
    formatter
      .formatToParts(date)
      .filter((part) => part.type !== "literal")
      .map((part) => [part.type, part.value]),
  )
  return `${parts.month}-${parts.day} ${parts.hour}:${parts.minute}`
}

/** 展示成员可见的会话历史和当前页面已发送消息。 */
export function ConversationTimeline({
  conversationID,
  conversationType,
  currentIdentityID,
  requireWindowFocus = true,
  workspaceLayout = false,
  outgoingMessages,
  onRetryFailedMessage,
  retryFailedMessageDisabled = false,
  onReplyMessage,
}: {
  conversationID: string
  conversationType: ConversationType
  currentIdentityID: string
  requireWindowFocus?: boolean
  workspaceLayout?: boolean
  outgoingMessages: OutgoingConversationMessage[]
  onRetryFailedMessage?: (message: OutgoingConversationDraft) => void
  retryFailedMessageDisabled?: boolean
  onReplyMessage?: (message: ConversationMessageReference) => void
}) {
  const { t, i18n } = useTranslation("inbox")
  const navigate = useNavigate()
  const timeZone = useUserTimeZone()
  const pollingActive = useMemberChatPollingActive({ requireWindowFocus })
  const scrollRootRef = useRef<HTMLDivElement>(null)
  const aliveRef = useRef(true)
  const beforeRequestRef = useRef(0)
  const pollingRequestRef = useRef(false)
  const timelineRef = useRef<ConversationMessageListData | null>(null)
  const initialScrollRef = useRef(true)
  const prependScrollHeightRef = useRef<number | null>(null)
  const handledPrependRevisionRef = useRef(0)
  const previousSentCountRef = useRef(0)
  const previousMessageCountRef = useRef(0)
  const nearBottomRef = useRef(true)
  const [timeline, setTimeline] =
    useState<ConversationMessageListData | null>(null)
  const [loadingEarlier, setLoadingEarlier] = useState(false)
  const [earlierError, setEarlierError] = useState(false)
  const [pollingError, setPollingError] = useState(false)
  const [newMessagesAvailable, setNewMessagesAvailable] = useState(false)
  const [prependRevision, setPrependRevision] = useState(0)
  const { data, loading, error, refresh } = useResource(
    resourceKeys.conversationMessages(conversationID),
    (signal) => listConversationMessages(conversationID, undefined, signal),
    { refetchOnWindowFocus: false },
  )
  const currentPage = timeline ?? data ?? null
  const visibleMessages = mergeTimelineMessages(
    currentPage?.messages ?? [],
    outgoingMessages,
  )

  /** 复制一条文本消息的正文。 */
  async function copyMessageText(body: string) {
    try {
      await navigator.clipboard.writeText(body)
      toast.success(t("messageCopySuccess"))
    } catch (copyError) {
      console.warn("复制消息文本失败", copyError)
      toast.error(t("messageCopyError"))
    }
  }

  const dateFormatters = useMemo(() => {
    const locale = i18n.resolvedLanguage
    return {
      clock: new Intl.DateTimeFormat(locale, {
        timeZone,
        hour: "2-digit",
        minute: "2-digit",
        hourCycle: "h23",
      }),
      sessionTime: new Intl.DateTimeFormat("en-US", {
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
      dayKey: new Intl.DateTimeFormat("en-CA", {
        timeZone,
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
      }),
      monthDay: new Intl.DateTimeFormat(locale, {
        timeZone,
        month: "long",
        day: "numeric",
      }),
      fullDate: new Intl.DateTimeFormat(locale, {
        timeZone,
        year: "numeric",
        month: "long",
        day: "numeric",
      }),
    }
  }, [i18n.resolvedLanguage, timeZone])

  /** 按用户时区显示时间线日期分隔。 */
  function formatDayLabel(date: Date) {
    const day = dateFormatters.dayKey.format(date)
    const today = dateFormatters.dayKey.format(new Date())
    if (day === today) return t("today")
    if (day === previousDayKey(today)) return t("yesterday")
    return day.slice(0, 4) === today.slice(0, 4)
      ? dateFormatters.monthDay.format(date)
      : dateFormatters.fullDate.format(date)
  }

  /** 按当前语言连接系统事件中的成员姓名。 */
  function formatGroupParticipantNames(names: string[]) {
    if (names.length < 2) return names[0] ?? ""
    if (names.length === 2) {
      return names.join(t("groupSystemListPairSeparator"))
    }
    const previousNames = names
      .slice(0, -1)
      .join(t("groupSystemListSeparator"))
    return `${previousNames}${t("groupSystemListFinalSeparator")}${names[names.length - 1]}`
  }

  /** 将类型化群聊系统事件转换为当前语言的时间线文案。 */
  function formatGroupSystemEvent(event: ConversationSystemEvent) {
    const participantName = (
      participant: ConversationSystemEventParticipant,
    ) =>
      participant.identityId === currentIdentityID
        ? t("messageSenderYou")
        : participant.displayName
    const actor = participantName(event.actor)
    const targets = formatGroupParticipantNames(
      (event.targets ?? []).map(participantName),
    )
    switch (event.type) {
      case ConversationSystemEventType.ConversationSystemEventGroupRenamed:
        return t("groupSystemRenamed", {
          actor,
          previousTitle: event.previousTitle,
          title: event.title,
        })
      case ConversationSystemEventType.ConversationSystemEventGroupMembersAdded:
        return t("groupSystemMembersAdded", { actor, targets })
      case ConversationSystemEventType.ConversationSystemEventGroupMemberRemoved:
        return t("groupSystemMemberRemoved", {
          actor,
          target: targets,
        })
      case ConversationSystemEventType.ConversationSystemEventGroupMemberLeft:
        return t("groupSystemMemberLeft", { actor })
      case ConversationSystemEventType.ConversationSystemEventGroupOwnerTransferred:
        return t("groupSystemOwnerTransferred", {
          actor,
          target: targets,
        })
      case ConversationSystemEventType.ConversationSystemEventGroupDissolved:
        return t("groupSystemDissolved", { actor })
      default:
        return t("groupSystemUpdated")
    }
  }

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
      beforeRequestRef.current += 1
    }
  }, [])

  useEffect(() => {
    if (!data) return
    setTimeline((current) =>
      current
        ? {
            messages: mergeMessages(current.messages, data.messages),
            before: current.before,
            after: current.after ?? data.after,
          }
        : data,
    )
  }, [data])

  useEffect(() => {
    timelineRef.current = timeline
  }, [timeline])

  /** 增量读取当前会话的新消息；空会话使用无游标最近页。 */
  const pollMessages = useCallback(async () => {
    const base = timelineRef.current
    if (!base || pollingRequestRef.current) return
    pollingRequestRef.current = true
    const after = base.after
    try {
      const page = await listConversationMessages(
        conversationID,
        after ? { before: "", after } : undefined,
      )
      if (!aliveRef.current) return
      setPollingError(false)
      setTimeline((current) => {
        if (!current || (after && current.after !== after)) return current
        if (page.messages.length === 0) return current
        const messages = mergeMessages(current.messages, page.messages)
        const nextAfter =
          page.after && page.after !== current.after
            ? page.after
            : current.after
        if (
          messages.length === current.messages.length &&
          nextAfter === current.after
        ) {
          return current
        }
        return {
          messages,
          before: current.before ?? page.before,
          after: nextAfter,
        }
      })
    } catch (pollError) {
      if (!aliveRef.current || recoverSession(pollError, navigate)) return
      console.warn("轮询成员会话消息失败", {
        conversationId: conversationID,
        error: pollError,
      })
      setPollingError(true)
    } finally {
      pollingRequestRef.current = false
    }
  }, [conversationID, navigate])

  const timelineReady = timeline !== null
  useEffect(() => {
    if (!pollingActive || !timelineReady) return
    void pollMessages()
    const timer = window.setInterval(
      () => void pollMessages(),
      memberChatPollingInterval,
    )
    return () => window.clearInterval(timer)
  }, [pollMessages, pollingActive, timelineReady])

  useEffect(() => {
    const viewport = timelineViewport(scrollRootRef.current)
    if (!viewport) return
    const activeViewport: HTMLElement = viewport

    /** 记录用户是否仍在查看时间线底部。 */
    function syncBottomState() {
      const nearBottom = timelineNearBottom(activeViewport)
      nearBottomRef.current = nearBottom
      if (nearBottom) setNewMessagesAvailable(false)
    }

    syncBottomState()
    activeViewport.addEventListener("scroll", syncBottomState, {
      passive: true,
    })
    return () =>
      activeViewport.removeEventListener("scroll", syncBottomState)
  }, [currentPage])

  useLayoutEffect(() => {
    const viewport = timelineViewport(scrollRootRef.current)
    if (!viewport) return
    const sentCountIncreased =
      outgoingMessages.length > previousSentCountRef.current
    const messageCountIncreased =
      visibleMessages.length > previousMessageCountRef.current
    const prependChanged =
      prependRevision !== handledPrependRevisionRef.current
    previousSentCountRef.current = outgoingMessages.length
    previousMessageCountRef.current = visibleMessages.length
    if (initialScrollRef.current && currentPage) {
      initialScrollRef.current = false
      viewport.scrollTop = viewport.scrollHeight
      nearBottomRef.current = true
      return
    }
    if (prependChanged) {
      handledPrependRevisionRef.current = prependRevision
      const previousScrollHeight = prependScrollHeightRef.current
      prependScrollHeightRef.current = null
      if (!sentCountIncreased && previousScrollHeight !== null) {
        viewport.scrollTop += viewport.scrollHeight - previousScrollHeight
        return
      }
    }
    if (sentCountIncreased) {
      viewport.scrollTop = viewport.scrollHeight
      nearBottomRef.current = true
      setNewMessagesAvailable(false)
      return
    }
    if (workspaceLayout && messageCountIncreased) {
      if (nearBottomRef.current) {
        viewport.scrollTop = viewport.scrollHeight
      } else {
        setNewMessagesAvailable(true)
      }
      return
    }
    if (workspaceLayout && nearBottomRef.current) {
      viewport.scrollTop = viewport.scrollHeight
    }
  }, [
    currentPage,
    outgoingMessages.length,
    prependRevision,
    visibleMessages.length,
    workspaceLayout,
  ])

  /** 滚动到最新消息。 */
  function scrollToLatest() {
    const viewport = timelineViewport(scrollRootRef.current)
    if (!viewport) return
    viewport.scrollTop = viewport.scrollHeight
    nearBottomRef.current = true
    setNewMessagesAvailable(false)
  }

  /** 加载并前插一页更早消息。 */
  async function loadEarlier() {
    const before = timelineRef.current?.before
    if (!before || loadingEarlier) return
    const request = beforeRequestRef.current + 1
    beforeRequestRef.current = request
    setLoadingEarlier(true)
    setEarlierError(false)
    try {
      const earlierPage = await listConversationMessages(conversationID, {
        before,
        after: "",
      })
      if (!aliveRef.current || beforeRequestRef.current !== request) return
      const base = timelineRef.current
      if (!base || base.before !== before) return
      const viewport = timelineViewport(scrollRootRef.current)
      prependScrollHeightRef.current = viewport?.scrollHeight ?? null
      setTimeline((current) => {
        if (!current || current.before !== before) return current
        return {
          messages: mergeMessages(earlierPage.messages, current.messages),
          before: earlierPage.before,
          after: current.after,
        }
      })
      setPrependRevision((current) => current + 1)
    } catch (requestError) {
      if (!aliveRef.current || beforeRequestRef.current !== request) return
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

  /** 判断相邻消息是否属于同一个紧凑展示组。 */
  function messagesShareGroup(
    previous: TimelineMessage | undefined,
    next: TimelineMessage | undefined,
  ) {
    if (
      !previous ||
      !next ||
      next.sessionStart ||
      previous.type === MessageType.MessageTypeSystem ||
      next.type === MessageType.MessageTypeSystem
    ) {
      return false
    }
    if (
      timelineSenderKey(previous, currentIdentityID) !==
      timelineSenderKey(next, currentIdentityID)
    ) {
      return false
    }
    if (
      dateFormatters.dayKey.format(new Date(previous.originatedAt)) !==
      dateFormatters.dayKey.format(new Date(next.originatedAt))
    ) {
      return false
    }
    const interval =
      Date.parse(next.originatedAt) - Date.parse(previous.originatedAt)
    return interval >= 0 && interval <= timelineGroupInterval
  }

  if (loading && !currentPage && outgoingMessages.length === 0) {
    return (
      <LoadingIndicator className="min-h-0 flex-1 justify-center bg-background">
        {t("messagesLoading")}
      </LoadingIndicator>
    )
  }

  if (error && !currentPage && outgoingMessages.length === 0) {
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

  if (visibleMessages.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center bg-background p-6 text-sm text-muted-foreground">
        {t("messagesEmpty")}
      </div>
    )
  }

  return (
    <div className="relative min-h-0 flex-1 bg-background">
      <ScrollArea
        ref={scrollRootRef}
        className={cn(
          "h-full min-h-0 bg-background",
          workspaceLayout &&
            "[&>[data-slot=scroll-area-viewport]>div]:!flex [&>[data-slot=scroll-area-viewport]>div]:!min-h-full [&>[data-slot=scroll-area-viewport]>div]:!flex-col",
        )}
      >
        <div
          className={cn(
            "flex w-full flex-col px-4 pb-3 md:px-6",
            workspaceLayout && "flex-1 justify-end",
          )}
        >
          {currentPage?.before || earlierError ? (
            <div className="flex items-center justify-center py-2">
              {currentPage?.before ? (
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

          <div className="flex flex-col">
            {visibleMessages.map((message, index) => {
              const previous = visibleMessages[index - 1]
              const next = visibleMessages[index + 1]
              const date = new Date(message.originatedAt)
              const day = dateFormatters.dayKey.format(date)
              const startsDay =
                workspaceLayout &&
                (!previous ||
                  dateFormatters.dayKey.format(
                    new Date(previous.originatedAt),
                  ) !== day)
              const startsGroup = workspaceLayout
                ? !messagesShareGroup(previous, message)
                : true
              const endsGroup = workspaceLayout
                ? !messagesShareGroup(message, next)
                : true
              const incoming = message.local
                ? false
                : conversationType !==
                    ConversationType.ConversationTypeCustomer
                  ? !message.sender ||
                    message.sender.sourceId !== currentIdentityID
                  : !message.sender ||
                    message.sender.kind ===
                      ChatSubjectKind.ChatSubjectKindContact
              const sentByCurrentIdentity =
                !message.local &&
                conversationType !==
                  ConversationType.ConversationTypeCustomer &&
                message.sender?.sourceId === currentIdentityID
              const senderName =
                (message.local || sentByCurrentIdentity
                  ? t("messageSenderYou")
                  : message.sender?.displayName?.trim()) ||
                (message.sender?.kind ===
                ChatSubjectKind.ChatSubjectKindContact
                  ? t("anonymousVisitor")
                  : t("unknownSender"))
              const senderInitial =
                Array.from(senderName)[0]?.toLocaleUpperCase() ?? "?"
              const failedDraft =
                message.deliveryStatus === "failed" &&
                message.clientMessageID
                  ? {
                      clientMessageID: message.clientMessageID,
                      body: message.body,
                      originatedAt: message.originatedAt,
                      replyTo: message.replyTo,
                      mentionSubjectIDs: message.mentionSubjectIDs,
                    }
                  : null
              const systemEvent = message.systemEvent
              const systemEventText = systemEvent
                ? formatGroupSystemEvent(systemEvent)
                : null

              return (
                <Fragment key={message.id}>
                  {startsDay ? (
                    <div className="my-3 flex items-center justify-center">
                      <time
                        dateTime={day}
                        className="rounded-full bg-muted px-3 py-1 text-[11px] font-medium text-muted-foreground"
                      >
                        {formatDayLabel(date)}
                      </time>
                    </div>
                  ) : null}
                  {message.sessionStart ? (
                    <div className="my-3 flex items-center gap-3 text-xs font-semibold text-foreground">
                      <span className="h-px flex-1 bg-border" />
                      <span className="rounded-full border border-primary bg-background px-3 py-1 text-primary">
                        {t("sessionBoundary", {
                          sequence: message.sessionStart.sequence,
                          time: formatMessageTime(
                            dateFormatters.sessionTime,
                            new Date(message.sessionStart.startedAt),
                          ),
                        })}{" "}
                        ·{" "}
                        {message.sessionStart.status ===
                        ServiceSessionStatus.ServiceSessionStatusClosed
                          ? t("sessionBoundaryClosed")
                          : t("sessionBoundaryOngoing")}
                      </span>
                      <span className="h-px flex-1 bg-border" />
                    </div>
                  ) : null}
                  {message.type === MessageType.MessageTypeSystem &&
                  systemEventText ? (
                    <div
                      className={cn(
                        "flex justify-center px-10 text-center",
                        index > 0 && "mt-3",
                      )}
                    >
                      <span
                        className="rounded-full bg-muted px-3 py-1 text-xs text-muted-foreground"
                        title={dateFormatters.full.format(date)}
                      >
                        {systemEventText}
                      </span>
                    </div>
                  ) : (
                    <ContextMenu>
                      <article
                        className={cn(
                          "flex items-start gap-2",
                          index > 0 && (startsGroup ? "mt-3" : "mt-1"),
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
                          {!workspaceLayout ? (
                            <time
                              dateTime={message.originatedAt}
                              title={dateFormatters.full.format(date)}
                              className="text-[11px] text-muted-foreground/80"
                            >
                              {formatMessageTime(
                                dateFormatters.sessionTime,
                                date,
                              )}
                            </time>
                          ) : null}
                          {conversationType ===
                            ConversationType.ConversationTypeGroup &&
                          (!workspaceLayout || incoming) &&
                          startsGroup ? (
                            <span className="max-w-full truncate text-xs font-medium text-foreground">
                              {senderName}
                            </span>
                          ) : null}
                          <div className="relative max-w-full">
                            {endsGroup ? (
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
                            ) : null}
                            <ContextMenuTrigger asChild>
                              <div className="group/message relative max-w-full">
                                {!message.local && onReplyMessage ? (
                                  <button
                                    type="button"
                                    className={cn(
                                        "pointer-events-none absolute top-0 z-10 rounded-md border bg-background px-2 py-1 text-xs text-foreground opacity-0 shadow-sm transition-opacity group-focus-within/message:pointer-events-auto group-focus-within/message:opacity-100 group-hover/message:pointer-events-auto group-hover/message:opacity-100 focus-visible:pointer-events-auto focus-visible:opacity-100",
                                        incoming ? "left-full" : "right-full",
                                    )}
                                    onClick={() =>
                                      onReplyMessage({
                                        id: message.id,
                                        body: message.body,
                                        sender: message.sender,
                                      })
                                    }
                                  >
                                    {t("messageReply")}
                                  </button>
                                ) : null}
                                <div
                                  className={cn(
                                    "min-w-0 max-w-full rounded-2xl px-3 py-2 text-sm break-words [overflow-wrap:anywhere]",
                                    incoming
                                      ? cn(
                                          "border bg-background text-foreground shadow-xs",
                                          endsGroup && "rounded-bl-sm",
                                        )
                                      : cn(
                                          "bg-primary text-primary-foreground",
                                          endsGroup && "rounded-br-sm",
                                        ),
                                  )}
                                >
                                  {message.replyTo ? (
                                    <div
                                      className={cn(
                                        "mb-1.5 border-l-2 pl-2 text-xs",
                                        incoming
                                          ? "border-primary text-muted-foreground"
                                          : "border-primary-foreground/60 text-primary-foreground/75",
                                      )}
                                    >
                                      <p className="font-medium">
                                        {message.replyTo.sender?.displayName?.trim() ||
                                          t("unknownSender")}
                                      </p>
                                      <p className="line-clamp-2 whitespace-pre-wrap">
                                        {message.replyTo.body}
                                      </p>
                                    </div>
                                  ) : null}
                                  <div
                                    className={cn(
                                      "min-w-0",
                                      workspaceLayout &&
                                        "flex items-end gap-2",
                                    )}
                                  >
                                    <span className="min-w-0 whitespace-pre-wrap">
                                      {renderMessageBody(message)}
                                    </span>
                                    {workspaceLayout ? (
                                      <time
                                        dateTime={message.originatedAt}
                                        title={dateFormatters.full.format(date)}
                                        className={cn(
                                          "shrink-0 translate-y-0.5 text-[10px]",
                                          incoming
                                            ? "text-muted-foreground"
                                            : "text-primary-foreground/75",
                                        )}
                                      >
                                        {dateFormatters.clock.format(date)}
                                      </time>
                                    ) : null}
                                  </div>
                                </div>
                              </div>
                            </ContextMenuTrigger>
                          </div>
                          {failedDraft && onRetryFailedMessage ? (
                            <div
                              className="flex items-center gap-1.5 text-[11px] text-destructive"
                              role="status"
                            >
                              <span>{t("messageSendError")}</span>
                              <button
                                type="button"
                                className="underline-offset-2 hover:underline disabled:cursor-not-allowed disabled:opacity-50 disabled:no-underline"
                                disabled={retryFailedMessageDisabled}
                                onClick={() =>
                                  onRetryFailedMessage(failedDraft)
                                }
                              >
                                {t("messageRetry")}
                              </button>
                            </div>
                          ) : message.deliveryStatus ? (
                            <span
                              className={cn(
                                "text-[11px]",
                                message.deliveryStatus === "failed"
                                  ? "text-destructive"
                                  : "text-muted-foreground",
                              )}
                              role={
                                message.deliveryStatus === "failed"
                                  ? "status"
                                  : undefined
                              }
                            >
                              {message.deliveryStatus === "failed"
                                ? t("messageSendError")
                                : t("messageSending")}
                            </span>
                          ) : null}
                        </div>
                      </article>
                      <ContextMenuContent>
                        {!message.local && onReplyMessage ? (
                          <ContextMenuItem
                            onSelect={() =>
                              onReplyMessage({
                                id: message.id,
                                body: message.body,
                                sender: message.sender,
                              })
                            }
                          >
                            {t("messageReply")}
                          </ContextMenuItem>
                        ) : null}
                        <ContextMenuItem
                          onSelect={() => void copyMessageText(message.body)}
                        >
                          {t("messageCopyText")}
                        </ContextMenuItem>
                      </ContextMenuContent>
                    </ContextMenu>
                  )}
                </Fragment>
              )
            })}
          </div>
        </div>
      </ScrollArea>
      {workspaceLayout && pollingError ? (
        <button
          type="button"
          className="absolute top-2 left-1/2 z-10 min-h-8 -translate-x-1/2 rounded-full border bg-background/95 px-3 text-xs text-warning shadow-sm backdrop-blur"
          onClick={() => void pollMessages()}
        >
          {t("messagesRefreshError")}
        </button>
      ) : null}
      {workspaceLayout && newMessagesAvailable ? (
        <Button
          type="button"
          size="sm"
          className="absolute bottom-3 left-1/2 z-10 -translate-x-1/2 rounded-full shadow-md"
          onClick={scrollToLatest}
        >
          {t("messagesNew")}
        </Button>
      ) : null}
    </div>
  )
}
