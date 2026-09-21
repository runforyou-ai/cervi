/** 展示各类会话的成员消息时间线、Agent 结果与发送状态。 */
import { type RefObject, useCallback, useEffect, useEffectEvent, useMemo, useRef } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  listCustomerMessageDeliveries,
  listConversationMessageReferences,
  AgentRunStatus,
  ChatSubjectKind,
  ConversationSystemEventType,
  ServiceSessionTargetKind,
  ConversationType,
  MessageType,
  MessageVisibility,
  OrganizationIdentityType,
  ServiceSessionStatus,
  isApiError,
  isNotFoundApiError,
  type CurrentUser,
  type ConversationMessageData,
  type ConversationMessageReference,
  type ConversationSystemEventParticipant,
} from "@/api"
import { CustomerDeliveryState } from "./customer-delivery-state"
import { ConversationAttachment } from "./conversation-attachment"
import { MessageSendState } from "./message-send-state"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { MessageMarkdown } from "@/components/message-markdown"
import { messagePreview } from "@/lib/message-preview"
import { resolveAppPlatform } from "@/platform/app-platform"
import { openExternalURL } from "@/platform/external-navigation"
import { ProfileAvatar } from "@/components/profile-avatar"
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
import { mentionTokenPattern } from "@/lib/mention-token"
import { resources, supportedLanguages } from "@/i18n/resources"
import { useMemberChatPollingActive } from "@/features/inbox/use-member-chat-polling"
import { useRealtimeSyncActive } from "@/contexts/realtime-sync-context"
import { publishConversationAgentActivity } from "@/features/inbox/conversation-agent-activity"
import {
  nextTypingArrival,
  type TypingArrivalBaseline,
} from "@/features/inbox/conversation-typing-arrival"
import { clearConversationTypingSender } from "@/features/inbox/use-conversation-typing"
import { useOutgoingMessageStore } from "@/features/inbox/outgoing-message-context"
import {
  coveredByWindow,
  windowCoverage,
  type OutgoingConversationDraft,
  type OutgoingConversationMessage,
} from "@/features/inbox/outgoing-message-store"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

import { compareConversationMessages } from "./conversation-window"
import { useConversationTimeline } from "./use-conversation-timeline"
import { useConversationViewport } from "./use-conversation-viewport"
import { useConversationReading } from "./use-conversation-reading"
import { useConversationMessageNavigation } from "./use-conversation-message-navigation"
import { useConversationMentionNavigation } from "./use-conversation-mention-navigation"
import { ConversationMentionNavigator } from "./conversation-mention-navigator"
import { AgentProcess, AgentQueueState, AgentRunState, handoffReasonKey } from "./agent-process"

type TimelineMessage = Pick<
  ConversationMessageData,
  | "id"
  | "type"
  | "visibility"
  | "body"
  | "attachment"
  | "originatedAt"
  | "sender"
  | "sessionStart"
  | "systemEvent"
  | "replyTo"
  | "canReply"
  | "canNoteReply"
  | "mentions"
  | "mentionAll"
  | "agentProcess"
> & {
  persistedMessageID: string | null
  clientMessageID: string | null
  draftMentions: OutgoingConversationDraft["mentions"]
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
) {
  const messages: TimelineMessage[] = [...current].sort(compareConversationMessages).map((message) => ({
    ...message,
    persistedMessageID: message.id,
    clientMessageID: null,
    draftMentions: [],
    mentionAllToken: null,
    local: false,
    deliveryStatus: null,
  }))
  const coverage = windowCoverage(current)
  for (const message of outgoing) {
    // 只为窗口之外的发送项生成本地气泡。
    if (coveredByWindow(message, coverage)) continue
    messages.push({
      id: `local:${message.clientMessageID}`,
      persistedMessageID: message.saved?.id ?? null,
      type: message.saved?.type ?? (message.attachment ? MessageType.MessageTypeAttachment : MessageType.MessageTypeText),
      visibility: message.saved?.visibility ?? message.visibility,
      attachment: message.saved?.attachment ?? message.attachment ?? null,
      body: message.body,
      originatedAt: message.originatedAt,
      sender: null,
      agentProcess: null,
      sessionStart: null,
      systemEvent: null,
      replyTo: message.replyTo,
      canReply: false,
      canNoteReply: false,
      mentions: message.mentions.map((mention) => ({
        chatSubjectId: mention.chatSubjectID ?? "",
        kind: ChatSubjectKind.ChatSubjectKindOrganizationIdentity,
        sourceId: mention.identityID,
        displayName: mention.displayName,
      })),
      clientMessageID: message.clientMessageID,
      draftMentions: message.mentions,
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
  return messages
}

/** 收集一条消息中结构化提醒的成员名称，长名称优先匹配。 */
function messageMentionNames(message: TimelineMessage) {
  return [
    ...message.mentions.map((mention) => mention.displayName?.trim() ?? ""),
    ...(message.mentionAll ? mentionAllNames : []),
  ]
    .filter((name, index, values) => name && values.indexOf(name) === index)
    .sort((left, right) => right.length - left.length)
}

/** 在消息正文中强调结构化提醒。 */
function renderMessageBody(message: TimelineMessage) {
  const names = messageMentionNames(message)
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

/** 展示 AI 给出的对客回复，并提供填入回复草稿的操作。 */
function CustomerReplyBlock({
  body,
  disabledReason,
  onApply,
}: {
  body: string
  disabledReason: string | null
  onApply: (body: string) => void
}) {
  const { t } = useTranslation("inbox")
  return (
    <div className="my-2 rounded-lg border bg-background p-2.5 text-foreground">
      <div className="whitespace-pre-wrap break-words">{body}</div>
      <div className="mt-2 flex justify-end">
        <Button
          type="button"
          variant="outline"
          size="xs"
          disabled={Boolean(disabledReason)}
          title={disabledReason ?? undefined}
          onClick={() => onApply(body)}
        >
          {t("copilotApplyReply")}
        </Button>
      </div>
    </div>
  )
}

/** 外部请求定位的消息，nonce 区分对同一消息的多次请求。 */
export type ConversationLocateTarget = { messageId: string; nonce: number }

/** 展示成员可见的会话历史和当前页面已发送消息。 */
function ConversationTimelineContent({
  conversationID,
  conversationType,
  currentUser,
  requireWindowFocus = true,
  customerDeliveries = false,
  outgoingMessages,
  onRetryFailedMessage,
  retryFailedMessageDisabled = false,
  onReplyMessage,
  noteReplyEnabled = false,
  customerReplyUnavailable = false,
  replyVisibility = MessageVisibility.MessageVisibilityCustomerVisible,
  onReadMessage,
  readThroughMessageID,
  prepareSendRef,
  mentionNavigation = true,
  onUnavailable,
  enabled = true,
  locateMessage = null,
  onApplyReply,
  applyReplyDisabledReason = null,
}: {
  conversationID: string
  conversationType: ConversationType
  currentUser: CurrentUser
  requireWindowFocus?: boolean
  customerDeliveries?: boolean
  outgoingMessages: OutgoingConversationMessage[]
  onRetryFailedMessage?: (message: OutgoingConversationDraft) => void
  retryFailedMessageDisabled?: boolean
  onReplyMessage?: (
    message: ConversationMessageReference,
    visibility: MessageVisibility,
  ) => void
  noteReplyEnabled?: boolean
  customerReplyUnavailable?: boolean
  replyVisibility?: MessageVisibility
  onReadMessage?: (messageID: string) => void
  readThroughMessageID?: string | null
  prepareSendRef?: RefObject<(() => Promise<boolean>) | null>
  mentionNavigation?: boolean
  onUnavailable?: () => void
  enabled?: boolean
  locateMessage?: ConversationLocateTarget | null
  onApplyReply?: (body: string) => void
  applyReplyDisabledReason?: string | null
}) {
  const currentIdentityID = currentUser.identityId
  // 右侧 AI 助手面板宽度有限，消息不展示头像，改在气泡上方标出发送者。
  const copilot = conversationType === ConversationType.ConversationTypeCopilot
  // 有文本发送中时禁用文本重试，附件重试由上传队列排队。
  const sendingText = outgoingMessages.some(
    (message) => message.status === "sending" && !message.attachment,
  )
  const textRetryDisabled = retryFailedMessageDisabled || sendingText
  // 移动端气泡禁止文本选择，消息菜单项使用触屏尺寸，回复入口只通过长按菜单提供。
  const mobile = resolveAppPlatform() === "mobile"
  const menuItemClassName = cn(mobile && "min-h-11")
  const { t, i18n } = useTranslation(["inbox", "common"])
  const navigate = useNavigate()
  const timeZone = useUserTimeZone()
  const pollingActive = useMemberChatPollingActive({ requireWindowFocus })
  const realtime = useRealtimeSyncActive()
  const scrollRootRef = useRef<HTMLDivElement>(null)
  const keepPositionRef = useRef<(() => void) | null>(null)
  const invalidate = useResourceInvalidator()
  // 渲染入口保持稳定，正文组件的缓存不因每次渲染失效。
  const renderCustomerReply = useCallback(
    (language: string, code: string) => language === "customer-reply"
      ? <CustomerReplyBlock body={code.trim()} disabledReason={applyReplyDisabledReason} onApply={onApplyReply!} />
      : undefined,
    [applyReplyDisabledReason, onApplyReply],
  )
  const timeline = useConversationTimeline({
    conversationID,
    enabled,
    pollingActive,
    keepPosition: keepPositionRef,
  })
  const currentPage = timeline.page
  const { loading, error, refresh } = timeline
  const outgoingStore = useOutgoingMessageStore()
  useEffect(() => {
    if (!currentPage) return
    // 窗口已收录的发送项从发送状态中删除。
    outgoingStore.reconcile(conversationID, currentPage.messages)
  }, [conversationID, currentPage, outgoingStore])
  const latestMessage = currentPage?.messages[currentPage.messages.length - 1]
  const latestMessageSeq = latestMessage?.messageSeq
  const latestSenderSubjectID = latestMessage?.sender?.chatSubjectId
  const windowLoaded = Boolean(currentPage)
  const typingArrivalRef = useRef<TypingArrivalBaseline>(null)
  useEffect(() => {
    // 新消息到达后不再显示其发送者正在输入；首次加载窗口与回看历史窗口都不算新消息。
    const arrival = nextTypingArrival(typingArrivalRef.current, {
      conversationID,
      loaded: windowLoaded,
      messageSeq: latestMessageSeq,
    })
    typingArrivalRef.current = arrival.baseline
    if (arrival.arrived && latestSenderSubjectID) clearConversationTypingSender(conversationID, latestSenderSubjectID)
  }, [conversationID, latestMessageSeq, latestSenderSubjectID, windowLoaded])
  const agentActivityNames = [
    ...(currentPage?.agentRuns ?? [])
      .filter((run) => run.status === AgentRunStatus.AgentRunStatusQueued || run.status === AgentRunStatus.AgentRunStatusRunning)
      .map((run) => run.agentName),
    ...(currentPage?.pendingAgents ?? []).map((agent) => agent.displayName),
  ].join("\u0000")
  useEffect(() => {
    // 会话头按时间线读到的运行与排队状态展示 AI 员工正在回复。
    publishConversationAgentActivity(conversationID, agentActivityNames ? agentActivityNames.split("\u0000") : [])
    return () => publishConversationAgentActivity(conversationID, [])
  }, [agentActivityNames, conversationID])
  const visibleMessages = mergeTimelineMessages(
    currentPage?.messages ?? [],
    timeline.mode === "latest" ? outgoingMessages : [],
  )
  // 使用窗口内持久消息编号查询投递，历史窗口也能刷新原有消息的状态。
  const deliveryMessageIDs = visibleMessages
    .flatMap((message) => message.persistedMessageID ? [message.persistedMessageID] : [])
    .sort()
    .join(",")
  const deliveries = useResource(
    resourceKeys.customerDeliveries(conversationID, deliveryMessageIDs),
    () => listCustomerMessageDeliveries(conversationID, deliveryMessageIDs),
    {
      enabled: enabled && customerDeliveries && Boolean(deliveryMessageIDs),
      keepPreviousData: true,
      refetchInterval: pollingActive && !realtime ? 2000 : false,
    },
  )
  const references = useResource(
    resourceKeys.conversationMessageReferences(conversationID, deliveryMessageIDs),
    () => listConversationMessageReferences(conversationID, deliveryMessageIDs),
    {
      enabled: enabled && customerDeliveries && Boolean(deliveryMessageIDs),
      keepPreviousData: true,
      refetchInterval: pollingActive && !realtime ? 2000 : false,
    },
  )
  const referencesByMessage = new Map(references.data?.states.map((state) => [state.messageId, state]))
  const deliveriesByMessage = new Map(deliveries.data?.deliveries.map((delivery) => [delivery.messageId, delivery]))
  const viewport = useConversationViewport({
    root: scrollRootRef,
    page: currentPage,
    mode: timeline.mode,
    switching: timeline.switching,
    visibleCount: visibleMessages.length,
    sentCount: outgoingMessages.length,
  })
  // 窗口重读在读取完成后同步合入，合入前通过最新的视口入口保存阅读位置。
  keepPositionRef.current = viewport.keepReadingPosition
  const location = useConversationMessageNavigation({
    root: scrollRootRef,
    page: currentPage,
    readingActive: pollingActive,
    openWindow: timeline.openWindow,
    cancelWindowUpdate: timeline.cancelWindowUpdate,
    viewport,
  })
  const reading = useConversationReading({
    root: scrollRootRef,
    page: currentPage,
    mode: timeline.mode,
    switching: timeline.switching || location.locating,
    readingActive: enabled && pollingActive,
    identityID: currentIdentityID,
    atBottom: viewport.atBottom,
    getAtBottom: viewport.getAtBottom,
    onReadMessage,
    readThroughMessageID,
  })

  /** 当前成员失去会话访问权时恢复到会话列表。 */
  const handleUnavailable = useCallback(() => {
    if (onUnavailable) {
      onUnavailable()
      return
    }
    void invalidate(resourceKeys.conversationSummary(conversationID))
  }, [conversationID, invalidate, onUnavailable])

  useEffect(() => {
    if (isNotFoundApiError(error) || isNotFoundApiError(timeline.refreshError)) handleUnavailable()
  }, [error, timeline.refreshError, handleUnavailable])

  const mentions = useConversationMentionNavigation({
    conversationID,
    enabled:
      enabled &&
      mentionNavigation &&
      (conversationType === ConversationType.ConversationTypeGroup ||
        conversationType === ConversationType.ConversationTypeCustomer),
    pollingActive,
    root: scrollRootRef,
    page: currentPage,
    switching: timeline.switching || location.locating,
    locate: location.locate,
    cancel: location.cancel,
    onUnavailable: handleUnavailable,
  })

  // 按分页边界判断后续消息，群聊同时比较导航态的最新序号。
  const windowLastSequence =
    currentPage?.messages[currentPage.messages.length - 1]?.messageSeq
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
      viewport.followLatest()
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
    viewport.followLatest,
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
        // 重读当前窗口，引用状态以服务端结果为准。
        void timeline.refresh()
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

  // 检索结果请求定位时，等首屏窗口就绪后复用引用跳转流程，同一请求只处理一次。
  const locatedNonceRef = useRef(0)
  const locateRequested = useEffectEvent((messageID: string) => {
    void followReference(messageID)
  })
  useEffect(() => {
    if (!locateMessage || !currentPage || locatedNonceRef.current === locateMessage.nonce) return
    locatedNonceRef.current = locateMessage.nonce
    locateRequested(locateMessage.messageId)
  }, [locateMessage, currentPage])

  /** 加载相邻历史页并保持可见消息位置。 */
  async function loadPage(direction: "before" | "after") {
    try {
      await timeline.loadPage(direction, viewport.preservePosition)
    } catch (error) {
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

  /** 客服处理周期去向的时间线文案：成员取名称快照，团队与公共队列按队列文案展示。 */
  function sessionTargetText(
    target: NonNullable<ConversationMessageData["systemEvent"]>["sessionTarget"],
  ) {
    if (target?.kind === ServiceSessionTargetKind.ServiceSessionTargetMember) {
      return target.identityId === currentIdentityID
        ? t("messageSenderYou")
        : (target.displayName ?? t("unknownSender"))
    }
    return target?.kind === ServiceSessionTargetKind.ServiceSessionTargetTeam
      ? t("handoffTargetTeam", { name: target.teamName ?? "" })
      : t("handoffTargetPublicQueue")
  }

  /** 将类型化系统事件转换为当前语言的时间线文案。 */
  function formatSystemEvent(
    event: NonNullable<ConversationMessageData["systemEvent"]>,
  ) {
    // 转人工事件按去向与原因码本地化，名称取事件写入时的快照。
    if (event.type === ConversationSystemEventType.ConversationSystemEventServiceSessionHandedOff) {
      return t("serviceSessionHandedOff", {
        agent: event.fromDisplayName ?? t("unknownSender"),
        target: sessionTargetText(event.sessionTarget),
        reason: t(handoffReasonKey(event.handoffReason)),
      })
    }
    // 退回队列事件没有操作人，只展示原负责人与退回去向。
    if (
      event.type ===
      ConversationSystemEventType.ConversationSystemEventServiceSessionReturned
    ) {
      return t("serviceSessionReturned", {
        from:
          event.fromIdentityId === currentIdentityID
            ? t("messageSenderYou")
            : (event.fromDisplayName ?? t("unknownSender")),
        target: sessionTargetText(event.sessionTarget),
      })
    }
    const participantName = (
      participant: ConversationSystemEventParticipant,
    ) =>
      participant.identityId === currentIdentityID
        ? t("messageSenderYou")
        : participant.displayName
    const actor = participantName(event.actor)
    const targets = formatGroupParticipantNames(
      event.targets.map(participantName),
    )
    switch (event.type) {
      case ConversationSystemEventType.ConversationSystemEventServiceSessionClaimed:
        return t("serviceSessionClaimed", { actor })
      case ConversationSystemEventType.ConversationSystemEventServiceSessionTakenOver:
        return t("serviceSessionTakenOver", {
          actor,
          from:
            event.fromIdentityId === currentIdentityID
              ? t("messageSenderYou")
              : (event.fromDisplayName ?? t("unknownSender")),
        })
      case ConversationSystemEventType.ConversationSystemEventServiceSessionTransferred:
        return t("serviceSessionTransferred", {
          actor,
          target: sessionTargetText(event.sessionTarget),
        })
      case ConversationSystemEventType.ConversationSystemEventServiceSessionClosed:
        return t("serviceSessionClosed", { actor })
      case ConversationSystemEventType.ConversationSystemEventServiceSessionReopened:
        return t("serviceSessionReopened", { actor })
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
      previous.visibility !== next.visibility ||
      previous.type === MessageType.MessageTypeSystem ||
      previous.type === MessageType.MessageTypeAgentError ||
      previous.type === MessageType.MessageTypeAgentCancelled ||
      next.type === MessageType.MessageTypeAgentError ||
      next.type === MessageType.MessageTypeAgentCancelled ||
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
            {t("common:actions.retry")}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="relative min-h-0 flex-1 bg-background">
      <ScrollArea
        ref={scrollRootRef}
        // 将 Radix Viewport 的内联 table 布局覆盖为块级布局。
        className="h-full min-h-0 bg-background [&>[data-slot=scroll-area-viewport]>div]:!flex [&>[data-slot=scroll-area-viewport]>div]:!min-h-full [&>[data-slot=scroll-area-viewport]>div]:!flex-col"
      >
        <div className="flex w-full flex-1 flex-col px-3.5 pb-2.5 md:px-5">
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
            {visibleMessages.map((storedMessage, index) => {
              // 引用状态独立刷新，保留当前窗口、正文位置和滚动上下文。
              const referenceState = referencesByMessage.get(storedMessage.id)
              const message = referenceState ? { ...storedMessage, canReply: referenceState.canReply, canNoteReply: referenceState.canNoteReply, replyTo: referenceState.replyTo } : storedMessage
              const previous = visibleMessages[index - 1]
              const next = visibleMessages[index + 1]
              const agentError = message.type === MessageType.MessageTypeAgentError
              const agentCancelled = message.type === MessageType.MessageTypeAgentCancelled
              const agentNotice = agentError || agentCancelled
              const date = new Date(message.originatedAt)
              const day = dateFormatters.dayKey.format(date)
              const startsDay =
                !previous ||
                dateFormatters.dayKey.format(
                  new Date(previous.originatedAt),
                ) !== day
              const startsGroup = !messagesShareGroup(previous, message)
              const endsGroup = !messagesShareGroup(message, next)
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
              // 从身份资料生成头像及默认头像。
              const useCurrentUserAvatar =
                message.local ||
                (message.sender?.kind ===
                  ChatSubjectKind.ChatSubjectKindOrganizationIdentity &&
                  message.sender.sourceId === currentIdentityID)
              const failedDraft =
                message.deliveryStatus === "failed" && message.clientMessageID
                  ? {
                      clientMessageID: message.clientMessageID,
                      visibility: message.visibility,
                      body: message.body,
                      originatedAt: message.originatedAt,
                      replyTo: message.replyTo,
                      mentions: message.draftMentions,
                      mentionAll: message.mentionAll,
                      mentionAllToken: message.mentionAllToken,
                    }
                  : null
              // 回复与复制使用同一份消息摘要。
              const referenceBody = message.body || message.attachment?.name || ""
              const internalNote =
                message.visibility === MessageVisibility.MessageVisibilityInternalOnly
              // 内部备注的重试不受对客发送资格限制，仍与发送中的文本互斥。
              const messageRetryDisabled = internalNote ? sendingText : textRetryDisabled
              // 成员发往外部渠道的文本与附件共用同一份投递状态。
              const renderDeliveryState = customerDeliveries && !agentNotice && !internalNote && (message.local || message.sender?.kind === ChatSubjectKind.ChatSubjectKindOrganizationIdentity) ? (className?: string) => (
                <CustomerDeliveryState
                  className={className}
                  conversationID={conversationID}
                  delivery={message.persistedMessageID ? deliveriesByMessage.get(message.persistedMessageID) : undefined}
                  loadingError={Boolean(message.persistedMessageID && deliveries.error)}
                  onRefresh={() => void deliveries.refresh()}
                  localFailed={!message.attachment && message.deliveryStatus === "failed"}
                  onRetryLocal={failedDraft && onRetryFailedMessage ? () => onRetryFailedMessage(failedDraft) : undefined}
                  retryLocalDisabled={messageRetryDisabled}
                />
              ) : undefined
              // 引用落入的输入模式：当前处于备注模式，或这条消息不能用于对客回复时，都写入内部备注。
              const quoteAsNote =
                noteReplyEnabled &&
                (replyVisibility === MessageVisibility.MessageVisibilityInternalOnly ||
                  customerReplyUnavailable ||
                  !message.canReply)
              const quoteDisabled = quoteAsNote
                ? !message.canNoteReply
                : !message.canReply
              const quoteVisibility = quoteAsNote
                ? MessageVisibility.MessageVisibilityInternalOnly
                : MessageVisibility.MessageVisibilityCustomerVisible
              // 文字气泡与附件气泡共用同一套方向配色和组尾圆角，内部备注使用区别于对客消息的常驻样式。
              const bubbleClassName = cn(
                internalNote
                  ? "border border-dashed border-amber-500/70 bg-amber-50 text-foreground dark:bg-amber-950/40"
                  : incoming || agentNotice
                    ? "border bg-muted text-foreground shadow-xs"
                    : "bg-accent text-accent-foreground",
                endsGroup && (incoming ? "rounded-bl-sm" : "rounded-br-sm"),
              )
              const systemEvent = message.systemEvent
              const systemEventText = systemEvent
                ? formatSystemEvent(systemEvent)
                : null

              return (
                <div key={message.id}>
                  {startsDay ? (
                    <div className="my-3 flex items-center gap-2.5 text-[11px] font-medium text-muted-foreground">
                      <span className="h-px flex-1 bg-border" />
                      <time dateTime={day}>{formatDayLabel(date)}</time>
                      <span className="h-px flex-1 bg-border" />
                    </div>
                  ) : null}
                  {message.sessionStart ? (
                    <div className="my-3 text-center text-xs font-semibold text-muted-foreground">
                      <span>
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
                    </div>
                  ) : null}
                  {message.type === MessageType.MessageTypeSystem &&
                  systemEventText ? (
                    <div
                      data-message-id={message.local ? undefined : message.id}
                      tabIndex={-1}
                      className="my-3 flex flex-col items-center gap-1 px-10 text-center text-xs text-muted-foreground"
                    >
                      <span title={dateFormatters.full.format(date)}>
                        {systemEventText}
                      </span>
                      {systemEvent?.reasonText ? (
                        <span className="max-w-full break-words">
                          {systemEvent.reasonText}
                        </span>
                      ) : null}
                    </div>
                  ) : (
                    <ContextMenu>
                      <article
                        data-message-id={message.local ? undefined : message.id}
                        tabIndex={-1}
                        className={cn(
                          location.highlightedID === message.id &&
                            "message-location-highlight",
                          "group/message-row flex items-start gap-2",
                          index > 0 && (startsGroup ? "mt-3" : "mt-1"),
                          incoming ? "justify-start" : "justify-end",
                        )}
                        aria-label={`${senderName} ${dateFormatters.full.format(date)}`}
                      >
                        <div
                          className={cn(
                            "flex min-w-0 max-w-[75%] flex-col gap-1",
                            message.agentProcess && "max-w-[min(36rem,85%)] sm:max-w-[min(36rem,75%)]",
                            incoming ? "items-start" : "items-end",
                            !copilot && (incoming ? "ml-9" : "mr-9"),
                          )}
                        >
                          {(copilot ||
                            (conversationType === ConversationType.ConversationTypeGroup && incoming)) &&
                          startsGroup ? (
                            <span className="max-w-full truncate text-xs font-medium text-foreground">
                              {senderName}
                            </span>
                          ) : null}
                          {internalNote && startsGroup ? (
                            <span className="max-w-full truncate text-xs font-medium text-amber-700 dark:text-amber-400">
                              {t("internalNoteSender", { name: senderName })}
                            </span>
                          ) : null}
                          <div className="relative min-w-0 max-w-full">
                            {endsGroup && !copilot ? (
                              <ProfileAvatar
                                title={senderName}
                                name={useCurrentUserAvatar
                                  ? currentUser.displayName
                                  : message.sender?.displayName}
                                imageURL={useCurrentUserAvatar
                                  ? currentUser.avatarUrl
                                  : message.sender?.avatarUrl}
                                fallback={message.sender?.identityType ===
                                  OrganizationIdentityType.OrganizationIdentityTypeAgent
                                  ? "agent"
                                  : "person"}
                                className={cn(
                                  "absolute bottom-0 size-7 text-xs",
                                  incoming ? "right-full mr-2" : "left-full ml-2",
                                )}
                              />
                            ) : null}
                            <ContextMenuTrigger asChild>
                              <div className={cn("group/message relative max-w-full", mobile && "select-none")}>
                                {incoming && !agentNotice && onReplyMessage && !mobile ? (
                                  <button
                                    type="button"
                                    disabled={quoteDisabled}
                                    className="disabled:cursor-not-allowed disabled:opacity-50 pointer-events-none absolute top-0 -right-2 z-10 -translate-y-1/2 whitespace-nowrap rounded-lg border bg-background px-2 py-1 text-xs text-foreground opacity-0 shadow-sm transition-opacity group-focus-within/message:pointer-events-auto group-focus-within/message:opacity-100 group-hover/message:pointer-events-auto group-hover/message:opacity-100 focus-visible:pointer-events-auto focus-visible:opacity-100"
                                    onClick={() =>
                                      onReplyMessage(
                                        {
                                          id: message.id,
                                          type: message.type,
                                          visibility: message.visibility,
                                          body: referenceBody,
                                          sender: message.sender,
                                          deleted: false,
                                        },
                                        quoteVisibility,
                                      )
                                    }
                                  >
                                    {t("messageReply")}
                                  </button>
                                ) : null}
                                <div
                                  className={cn(
                                    "min-w-0 max-w-full text-sm break-words [overflow-wrap:anywhere]",
                                    !message.attachment && cn("rounded-2xl px-3 py-2", bubbleClassName),
                                  )}
                                >
                                  {message.replyTo ? (
                                    <button
                                      type="button"
                                      disabled={message.replyTo.deleted || !message.replyTo.id}
                                      onClick={() =>
                                        void followReference(
                                          message.replyTo!.id,
                                        )
                                      }
                                      className={cn(
                                        "mb-1.5 block w-full border-l-2 pl-2 text-left text-xs focus-visible:outline focus-visible:outline-2",
                                        // 内部备注与附件消息的引用块在对客气泡外，按所在背景取色。
                                        incoming || internalNote || message.attachment
                                          ? "border-primary text-muted-foreground"
                                          : "border-accent-foreground/60 text-accent-foreground/75",
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
                                            {message.replyTo.sender?.displayName?.trim() || message.replyTo.externalSenderName ||
                                              t(message.replyTo.sender?.kind === ChatSubjectKind.ChatSubjectKindContact ? "anonymousVisitor" : "unknownSender")}
                                          </span>
                                          <span className="line-clamp-2 whitespace-pre-wrap">
                                            {message.replyTo.body ? messagePreview(message.replyTo.body, message.replyTo.sender?.identityType) : t("messageOriginalUnavailable")}
                                          </span>
                                        </>
                                      )}
                                    </button>
                                  ) : null}
                                  {message.agentProcess ? (
                                    <AgentProcess process={message.agentProcess} incoming={incoming} onPrimary={!incoming && !agentNotice} onToggle={viewport.stopFollowing} />
                                  ) : null}
                                  {/* 时间跟随正文末行，正文按整行宽度排版。 */}
                                  <div
                                    className="cervi-message-body relative min-w-0 after:block after:clear-both after:content-['']"
                                    data-delivery={Boolean(renderDeliveryState || message.deliveryStatus) || undefined}
                                  >
                                    {agentNotice ? (
                                      <span className={agentError ? "text-destructive" : "text-muted-foreground"}>{t(agentError ? "agentRunFailed" : "agentReplyStopped")}</span>
                                    ) : message.attachment ? (
                                      <ConversationAttachment retryDisabled={retryFailedMessageDisabled} body={message.body} attachment={message.attachment} conversationID={conversationID} messageID={message.persistedMessageID ?? message.id}
                                        originatedAt={message.originatedAt} timeLabel={dateFormatters.clock.format(date)} timeTitle={dateFormatters.full.format(date)} incoming={incoming} bubbleClassName={bubbleClassName} renderDeliveryState={renderDeliveryState} />
                                    ) : message.sender?.identityType === OrganizationIdentityType.OrganizationIdentityTypeAgent ? (
                                      <div className="min-w-0">
                                        <MessageMarkdown
                                          locale={i18n.language}
                                          mentions={messageMentionNames(message)}
                                          onOpenLink={openExternalURL}
                                          renderCodeBlock={onApplyReply ? renderCustomerReply : undefined}
                                        >
                                          {message.body}
                                        </MessageMarkdown>
                                      </div>
                                    ) : (
                                      <span className="whitespace-pre-wrap">{renderMessageBody(message)}</span>
                                    )}
                                    {!message.attachment ? <div
                                      className={cn(
                                        "cervi-message-time float-right ml-2 inline-flex translate-y-0.5 items-center gap-1 whitespace-nowrap text-[10px]",
                                        incoming || agentNotice || internalNote
                                          ? "text-muted-foreground"
                                          : "text-accent-foreground/75",
                                      )}
                                    >
                                      <time
                                        dateTime={message.originatedAt}
                                        title={dateFormatters.full.format(date)}
                                      >
                                        {dateFormatters.clock.format(date)}
                                      </time>
                                      {renderDeliveryState ? renderDeliveryState()
                                      : message.deliveryStatus ? (
                                        <div className="inline-flex items-center gap-1.5 text-[11px]">
                                          <MessageSendState
                                            state={message.deliveryStatus === "failed" ? "attention" : "sending"}
                                            detail={message.deliveryStatus === "failed" ? t("messageSendError") : undefined}
                                          />
                                          {failedDraft && onRetryFailedMessage ? (
                                            <button
                                              type="button"
                                              className="underline-offset-2 hover:underline disabled:cursor-not-allowed disabled:opacity-50 disabled:no-underline"
                                              disabled={messageRetryDisabled}
                                              onClick={() => onRetryFailedMessage(failedDraft)}
                                            >
                                              {t("messageRetry")}
                                            </button>
                                          ) : null}
                                        </div>
                                      ) : null}
                                    </div> : null}
                                  </div>
                                </div>
                              </div>
                            </ContextMenuTrigger>
                          </div>
                        </div>
                      </article>
                      <ContextMenuContent>
                        {!message.local && !agentNotice && onReplyMessage ? (
                          <ContextMenuItem
                            className={menuItemClassName}
                            disabled={quoteDisabled}
                            onSelect={() =>
                              onReplyMessage(
                                {
                                  id: message.id,
                                  type: message.type,
                                  visibility: message.visibility,
                                  body: referenceBody,
                                  sender: message.sender,
                                  deleted: false,
                                },
                                quoteVisibility,
                              )
                            }
                          >
                            {t("messageReply")}
                          </ContextMenuItem>
                        ) : null}
                        <ContextMenuItem
                          className={menuItemClassName}
                          onSelect={() => void copyMessageText(agentNotice ? t(agentError ? "agentRunFailed" : "agentReplyStopped") : referenceBody)}
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
          {timeline.mode === "latest" && !currentPage?.hasLater
            ? (currentPage?.agentRuns ?? []).map((run) => (
              <AgentRunState
                key={run.id}
                run={run}
                onStopped={timeline.refresh}
                conversationID={
                  conversationType === ConversationType.ConversationTypeAgent ||
                  conversationType === ConversationType.ConversationTypeGroup ||
                  conversationType === ConversationType.ConversationTypeCopilot
                    ? conversationID
                    : undefined
                }
                group={conversationType === ConversationType.ConversationTypeGroup}
                copilot={conversationType === ConversationType.ConversationTypeCopilot}
                incoming={conversationType !== ConversationType.ConversationTypeCustomer}
                onToggle={viewport.stopFollowing}
              />
            ))
            : null}
          {timeline.mode === "latest" && !currentPage?.hasLater ? (
            <AgentQueueState
              agents={currentPage?.pendingAgents ?? []}
              incoming={conversationType !== ConversationType.ConversationTypeCustomer}
            />
          ) : null}
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
      {timeline.refreshError || (error && !currentPage) ? (
        <button
          type="button"
          className="absolute top-2 left-1/2 z-10 min-h-8 -translate-x-1/2 rounded-full border bg-background/95 px-3 text-xs text-warning shadow-sm backdrop-blur"
          disabled={loading}
          onClick={() => void refresh()}
        >
          {currentPage
            ? t("messagesRefreshError")
            : `${t("messagesLoadError")} · ${t("common:actions.retry")}`}
        </button>
      ) : null}
      <ConversationMentionNavigator
        navigation={mentions}
        showLatest={!viewport.atBottom || hasLaterMessages}
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
