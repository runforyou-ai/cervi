/** 客服、单聊与群聊共用的成员消息时间线。 */
import { useCallback, useEffect, useMemo, useRef, type RefObject } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearchParams } from "react-router"
import { toast } from "sonner"

import {
  ChatSubjectKind,
  ConversationSystemEventType,
  ConversationType,
  MessageType,
  ServiceSessionStatus,
  isApiError,
  type ConversationMessageData,
  type ConversationMessageReference,
  type ConversationSystemEvent,
  type ConversationSystemEventParticipant,
  type GroupParticipant,
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
import { resources, supportedLanguages } from "@/i18n/resources"
import { useMemberChatPollingActive } from "@/features/inbox/use-member-chat-polling"
import type {
  OutgoingConversationDraft,
  OutgoingConversationMessage,
} from "@/features/inbox/use-outgoing-conversation-messages"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

import { compareConversationMessages } from "./conversation-window"
import { useConversationTimeline } from "./use-conversation-timeline"
import { useConversationReading } from "./use-conversation-reading"
import { useConversationMessageNavigation } from "./use-conversation-message-navigation"
import { useConversationMentionNavigation } from "./use-conversation-mention-navigation"
import { ConversationMentionNavigator } from "./conversation-mention-navigator"

type TimelineMessage = Pick<
  ConversationMessageData,
  | "id"
  | "type"
  | "body"
  | "originatedAt"
  | "sourceOrder"
  | "groupMessageSequence"
  | "sender"
  | "sessionStart"
  | "systemEvent"
  | "replyTo"
  | "mentions"
  | "mentionAll"
> & {
  clientMessageID: string | null
  mentionSubjectIDs: string[]
  mentionAllToken: OutgoingConversationDraft["mentionAllToken"]
  local: boolean
  deliveryStatus: "sending" | "failed" | null
}

const timelineGroupInterval = 5 * 60 * 1000
const mentionAllNames = supportedLanguages.map(
  (language) => resources[language].inbox.messageMentionAll,
)

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
  groupParticipants: GroupParticipant[] = [],
) {
  const participantsBySubjectID = new Map(
    groupParticipants.map((participant) => [
      participant.chatSubjectId,
      participant,
    ]),
  )
  const messages: TimelineMessage[] = current.map((message) => ({
    ...message,
    clientMessageID: null,
    mentionSubjectIDs: [],
    mentionAllToken: null,
    local: false,
    deliveryStatus: null,
  }))
  for (const message of outgoing) {
    if (
      message.saved &&
      current.some((saved) => saved.id === message.saved?.id)
    )
      continue
    messages.push({
      id: `local:${message.clientMessageID}`,
      type: MessageType.MessageTypeText,
      body: message.body,
      originatedAt: message.originatedAt,
      sourceOrder: 0,
      groupMessageSequence: null,
      sender: null,
      sessionStart: null,
      systemEvent: null,
      replyTo: message.replyTo,
      mentions: message.mentionSubjectIDs.flatMap((subjectID) => {
        const participant = participantsBySubjectID.get(subjectID)
        return participant
          ? [
              {
                chatSubjectId: participant.chatSubjectId,
                kind: ChatSubjectKind.ChatSubjectKindOrganizationIdentity,
                sourceId: participant.identityId,
                displayName: participant.displayName,
              },
            ]
          : []
      }),
      clientMessageID: message.clientMessageID,
      mentionSubjectIDs: message.mentionSubjectIDs,
      mentionAll: message.mentionAll,
      mentionAllToken: message.mentionAllToken,
      local: true,
      deliveryStatus:
        message.status === "failed"
          ? "failed"
          : message.showSending && !message.saved
            ? "sending"
            : null,
    })
  }
  // 服务端消息只来自连续窗口，尚未补入窗口的发送结果继续作为本地项目展示。
  return messages.sort((left, right) => {
    if (left.local !== right.local) return left.local ? 1 : -1
    return compareConversationMessages(left, right)
  })
}

/** 在消息正文中强调结构化提醒。 */
function renderMessageBody(message: TimelineMessage) {
  const names = [
    ...message.mentions.map((mention) => mention.displayName?.trim() ?? ""),
    ...(message.mentionAll ? mentionAllNames : []),
  ]
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
function ConversationTimelineContent({
  conversationID,
  conversationType,
  currentIdentityID,
  requireWindowFocus = true,
  workspaceLayout = false,
  outgoingMessages,
  onRetryFailedMessage,
  retryFailedMessageDisabled = false,
  onReplyMessage,
  groupParticipants,
  onReadMessage,
  readThroughMessageID,
  prepareSendRef,
  enabled = true,
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
  groupParticipants?: GroupParticipant[]
  onReadMessage?: (messageID: string) => void
  readThroughMessageID?: string | null
  prepareSendRef?: RefObject<(() => Promise<boolean>) | null>
  enabled?: boolean
}) {
  const { t, i18n } = useTranslation("inbox")
  const navigate = useNavigate()
  const timeZone = useUserTimeZone()
  const pollingActive = useMemberChatPollingActive({ requireWindowFocus })
  const scrollRootRef = useRef<HTMLDivElement>(null)
  const [, setSearchParams] = useSearchParams()
  const timeline = useConversationTimeline(
    conversationID,
    pollingActive,
    enabled,
  )
  const currentPage = timeline.page
  const { loading, error, refresh } = timeline
  const visibleMessages = mergeTimelineMessages(
    currentPage?.messages ?? [],
    timeline.mode === "latest" ? outgoingMessages : [],
    groupParticipants,
  )
  const reading = useConversationReading({
    root: scrollRootRef,
    page: currentPage,
    mode: timeline.mode,
    switching: timeline.switching,
    readingActive: pollingActive,
    identityID: currentIdentityID,
    visibleCount: visibleMessages.length,
    sentCount: outgoingMessages.length,
    onReadMessage,
    readThroughMessageID,
  })
  const location = useConversationMessageNavigation({
    root: scrollRootRef,
    page: currentPage,
    readingActive: pollingActive,
    openWindow: timeline.openWindow,
    cancelWindowUpdate: timeline.cancelWindowUpdate,
  })

  /** 当前成员失去会话访问权时恢复到会话列表。 */
  const handleUnavailable = useCallback(() => {
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current)
        next.delete("conversation")
        return next
      },
      { replace: true },
    )
  }, [setSearchParams])
  const mentions = useConversationMentionNavigation({
    conversationID,
    enabled: conversationType === ConversationType.ConversationTypeGroup,
    pollingActive,
    root: scrollRootRef,
    page: currentPage,
    switching: timeline.switching || location.locating,
    locate: location.locate,
    cancel: location.cancel,
    onUnavailable: handleUnavailable,
  })

  // 区分当前窗口底部与会话尾端，历史浏览意图不决定按钮是否显示。
  const windowLastSequence =
    currentPage?.messages[currentPage.messages.length - 1]?.groupMessageSequence
  const hasLaterMessages = Boolean(
    currentPage?.hasLater ||
    (windowLastSequence &&
      BigInt(mentions.latestSequence) > BigInt(windowLastSequence)),
  )

  /** 读取最新窗口成功后结束本轮并恢复贴底。 */
  const returnToLatest = useCallback(async () => {
    mentions.pause()
    try {
      if (!(await timeline.openWindow())) return false
      mentions.close()
      reading.followLatest()
      return true
    } catch (error) {
      if (isApiError(error) && error.reason === "conversation_unavailable")
        handleUnavailable()
      else if (!recoverSession(error, navigate))
        toast.error(
          error instanceof Error ? error.message : t("messagesLoadError"),
        )
      return false
    }
  }, [
    mentions.pause,
    mentions.close,
    timeline.openWindow,
    reading.followLatest,
    handleUnavailable,
    navigate,
    t,
  ])

  useEffect(() => {
    if (!prepareSendRef) return
    prepareSendRef.current = () =>
      timeline.mode === "latest" && !timeline.switching
        ? Promise.resolve(true)
        : returnToLatest()
    return () => {
      prepareSendRef.current = null
    }
  }, [prepareSendRef, returnToLatest, timeline.mode, timeline.switching])

  /** 引用跳转暂停提及确认，失败保留原窗口。 */
  async function followReference(messageID: string) {
    mentions.pause()
    try {
      await location.locate(messageID)
    } catch (error) {
      if (isApiError(error) && error.reason === "message_unavailable") {
        timeline.markReferenceUnavailable(messageID)
        toast.message(t("messageOriginalDeleted"))
      } else if (
        isApiError(error) &&
        error.reason === "conversation_unavailable"
      )
        handleUnavailable()
      else if (!recoverSession(error, navigate))
        toast.error(
          error instanceof Error ? error.message : t("messagesLoadError"),
        )
    }
  }

  /** 加载相邻历史页并保持可见消息位置。 */
  async function loadPage(direction: "before" | "after") {
    reading.preservePosition()
    try {
      await timeline.loadPage(direction)
    } catch (error) {
      reading.cancelPreservedPosition()
      if (isApiError(error) && error.reason === "conversation_unavailable")
        handleUnavailable()
      else if (!recoverSession(error, navigate))
        toast.error(
          t(
            direction === "before"
              ? "messagesLoadEarlierError"
              : "messagesLoadLaterError",
          ),
        )
    }
  }

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
    const previousNames = names.slice(0, -1).join(t("groupSystemListSeparator"))
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
          {currentPage?.hasEarlier || timeline.pageError === "before" ? (
            <div className="flex items-center justify-center py-2">
              {currentPage?.hasEarlier ? (
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={
                    Boolean(timeline.loadingDirection) || timeline.switching
                  }
                  onClick={() => void loadPage("before")}
                >
                  {timeline.loadingDirection === "before"
                    ? t("messagesLoadingEarlier")
                    : t("messagesLoadEarlier")}
                </Button>
              ) : null}
              {timeline.pageError === "before" ? (
                <span className="ml-2 text-xs text-destructive" role="status">
                  {t("messagesLoadEarlierError")}
                </span>
              ) : null}
            </div>
          ) : null}

          {visibleMessages.length === 0 ? (
            <div className="p-6 text-center text-sm text-muted-foreground">
              {t("messagesEmpty")}
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
                : conversationType !== ConversationType.ConversationTypeCustomer
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
                (message.sender?.kind === ChatSubjectKind.ChatSubjectKindContact
                  ? t("anonymousVisitor")
                  : t("unknownSender"))
              const senderInitial =
                Array.from(senderName)[0]?.toLocaleUpperCase() ?? "?"
              const failedDraft =
                message.deliveryStatus === "failed" && message.clientMessageID
                  ? {
                      clientMessageID: message.clientMessageID,
                      body: message.body,
                      originatedAt: message.originatedAt,
                      replyTo: message.replyTo,
                      mentionSubjectIDs: message.mentionSubjectIDs,
                      mentionAll: message.mentionAll,
                      mentionAllToken: message.mentionAllToken,
                    }
                  : null
              const systemEvent = message.systemEvent
              const systemEventText = systemEvent
                ? formatGroupSystemEvent(systemEvent)
                : null

              return (
                <div key={message.id}>
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
                      data-message-id={message.local ? undefined : message.id}
                      tabIndex={-1}
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
                        data-message-id={message.local ? undefined : message.id}
                        tabIndex={-1}
                        className={cn(
                          location.highlightedID === message.id &&
                            "message-location-highlight",
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
                                {incoming && onReplyMessage ? (
                                  <button
                                    type="button"
                                    className="pointer-events-none absolute top-0 -right-2 z-10 -translate-y-1/2 whitespace-nowrap rounded-lg border bg-background px-2 py-1 text-xs text-foreground opacity-0 shadow-sm transition-opacity group-focus-within/message:pointer-events-auto group-focus-within/message:opacity-100 group-hover/message:pointer-events-auto group-hover/message:opacity-100 focus-visible:pointer-events-auto focus-visible:opacity-100"
                                    onClick={() =>
                                      onReplyMessage({
                                        id: message.id,
                                        body: message.body,
                                        sender: message.sender,
                                        deleted: false,
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
                                          "border bg-muted text-foreground shadow-xs",
                                          endsGroup && "rounded-bl-sm",
                                        )
                                      : cn(
                                          "bg-primary text-primary-foreground",
                                          endsGroup && "rounded-br-sm",
                                        ),
                                  )}
                                >
                                  {message.replyTo ? (
                                    <button
                                      type="button"
                                      disabled={message.replyTo.deleted}
                                      onClick={() =>
                                        void followReference(
                                          message.replyTo!.id,
                                        )
                                      }
                                      className={cn(
                                        "mb-1.5 block w-full border-l-2 pl-2 text-left text-xs focus-visible:outline focus-visible:outline-2",
                                        incoming
                                          ? "border-primary text-muted-foreground"
                                          : "border-primary-foreground/60 text-primary-foreground/75",
                                      )}
                                      aria-label={
                                        message.replyTo.deleted
                                          ? t("messageOriginalDeleted")
                                          : t("messageGoToOriginal")
                                      }
                                    >
                                      {message.replyTo.deleted ? (
                                        t("messageOriginalDeleted")
                                      ) : (
                                        <>
                                          <span className="block font-medium">
                                            {message.replyTo.sender?.displayName?.trim() ||
                                              t("unknownSender")}
                                          </span>
                                          <span className="line-clamp-2 whitespace-pre-wrap">
                                            {message.replyTo.body}
                                          </span>
                                        </>
                                      )}
                                    </button>
                                  ) : null}
                                  <div
                                    className={cn(
                                      "min-w-0",
                                      workspaceLayout && "flex items-end gap-2",
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
                                deleted: false,
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
                </div>
              )
            })}
          </div>
          {currentPage?.hasLater && timeline.mode === "anchor" ? (
            <div className="flex justify-center py-2">
              <Button
                size="sm"
                variant="ghost"
                disabled={
                  Boolean(timeline.loadingDirection) || timeline.switching
                }
                onClick={() => void loadPage("after")}
              >
                {timeline.loadingDirection === "after"
                  ? t("messagesLoadingLater")
                  : t("messagesLoadLater")}
              </Button>
              {timeline.pageError === "after" ? (
                <span className="ml-2 text-xs text-destructive" role="status">
                  {t("messagesLoadLaterError")}
                </span>
              ) : null}
            </div>
          ) : null}
        </div>
      </ScrollArea>
      {workspaceLayout &&
      timeline.pollingError &&
      timeline.mode === "latest" ? (
        <button
          type="button"
          className="absolute top-2 left-1/2 z-10 min-h-8 -translate-x-1/2 rounded-full border bg-background/95 px-3 text-xs text-warning shadow-sm backdrop-blur"
          onClick={() => void timeline.poll()}
        >
          {t("messagesRefreshError")}
        </button>
      ) : null}
      <ConversationMentionNavigator
        navigation={mentions}
        showLatest={
          !reading.atBottom ||
          hasLaterMessages ||
          (timeline.mode === "latest" && reading.newCount > 0)
        }
        newCount={timeline.mode === "latest" ? reading.newCount : 0}
        busy={timeline.switching}
        onLatest={() => void returnToLatest()}
      />
    </div>
  )
}

/** 切换会话时重新建立独立的窗口、定位和阅读状态。 */
export function ConversationTimeline(
  props: Parameters<typeof ConversationTimelineContent>[0],
) {
  return <ConversationTimelineContent key={props.conversationID} {...props} />
}
