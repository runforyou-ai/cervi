/** 展示各类会话的成员消息时间线、Agent 结果与发送状态。 */
import { type RefObject, useCallback, useEffect, useEffectEvent, useMemo, useRef } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ConversationSystemEventType,
  ConversationType,
  MessageVisibility,
  isApiError,
  isNotFoundApiError,
  type CurrentUser,
  type ConversationMessageReference,
} from "@/api"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useUserTimeZone } from "@/contexts/user-preferences"
import { useMemberChatPollingActive } from "@/features/inbox/use-member-chat-polling"
import type {
  OutgoingConversationDraft,
  OutgoingConversationMessage,
} from "@/features/inbox/outgoing-message-store"
import { recoverSession } from "@/lib/session-navigation"
import { resolveAppPlatform } from "@/platform/app-platform"

import { useConversationTimeline } from "./use-conversation-timeline"
import { useConversationViewport } from "./use-conversation-viewport"
import { useConversationReading } from "./use-conversation-reading"
import { useConversationMessageNavigation } from "./use-conversation-message-navigation"
import { useConversationMentionNavigation } from "./use-conversation-mention-navigation"
import { ConversationMentionNavigator } from "./conversation-mention-navigator"
import { AgentQueueState, AgentRunState } from "./agent-process"
import { createTimelineDateFormatters } from "./timeline-grouping"
import { TimelineMessageRow } from "./timeline-message-row"
import { mergeTimelineMessages } from "./timeline-messages"
import { useTimelineMessageStates } from "./use-timeline-message-states"
import { useTimelinePageSync } from "./use-timeline-page-sync"

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
  const mobile = resolveAppPlatform() === "mobile"
  return (
    <div className="my-2 rounded-lg border bg-background p-2.5 text-foreground">
      {/* 桌面端按钮右浮动在正文末尾，末行剩余宽度足够时同行展示；移动端按钮在正文下方占满整行。 */}
      <div className="flow-root whitespace-pre-wrap break-words">
        {body}
        <Button
          type="button"
          variant="outline"
          size={mobile ? "default" : "xs"}
          className={mobile ? "mt-2.5 min-h-11 w-full" : "float-right -mt-0.5 ml-2"}
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

const pageLoaderCopy = {
  before: {
    className: "flex items-center justify-center py-2",
    idle: "messagesLoadEarlier",
    loading: "messagesLoadingEarlier",
    error: "messagesLoadEarlierError",
  },
  after: {
    className: "flex justify-center py-2",
    idle: "messagesLoadLater",
    loading: "messagesLoadingLater",
    error: "messagesLoadLaterError",
  },
} as const

/** 展示加载更早或更晚消息的按钮与失败提示。 */
function TimelinePageLoader({
  direction,
  canLoad,
  loading,
  failed,
  disabled,
  onLoad,
}: {
  direction: "before" | "after"
  canLoad: boolean
  loading: boolean
  failed: boolean
  disabled: boolean
  onLoad: () => void
}) {
  const { t } = useTranslation(["inbox", "common"])
  const copy = pageLoaderCopy[direction]
  return (
    <div className={copy.className}>
      {canLoad ? (
        <Button
          size="sm"
          variant="ghost"
          disabled={disabled}
          onClick={onLoad}
        >
          {loading ? t(copy.loading) : t(copy.idle)}
        </Button>
      ) : null}
      {failed ? (
        <span className="ml-2 text-xs text-destructive" role="status">
          {t(copy.error)}
        </span>
      ) : null}
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
  replyVisibility = MessageVisibility.MessageVisibilityShared,
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
  // 有文本发送中时，消息行禁用文本重试。
  const sendingText = outgoingMessages.some(
    (message) => message.status === "sending" && !message.attachment,
  )
  const { t, i18n } = useTranslation(["inbox", "common"])
  const navigate = useNavigate()
  const timeZone = useUserTimeZone()
  const pollingActive = useMemberChatPollingActive({ requireWindowFocus })
  // 会话显示在可见窗口中即推进已读，不要求窗口获得焦点。
  const readingActive = useMemberChatPollingActive({ requireWindowFocus: false })
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
  useTimelinePageSync(conversationID, currentPage)
  const visibleMessages = mergeTimelineMessages(
    currentPage?.messages ?? [],
    timeline.mode === "latest" ? outgoingMessages : [],
  )
  const { deliveries, deliveriesByMessage, referencesByMessage } = useTimelineMessageStates({
    conversationID,
    messages: visibleMessages,
    enabled: enabled && customerDeliveries,
    pollingActive,
  })
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
    readingActive: enabled && readingActive,
    identityID: currentIdentityID,
    atBottom: viewport.atBottom,
    getAtBottom: viewport.getAtBottom,
    onReadMessage,
    readThroughMessageID,
  })
  // 客户会话中每个周期最后一次关闭事件承载该周期的小结。
  const summaryEventIDs = new Set<string>()
  if (conversationType === ConversationType.ConversationTypeChannel) {
    const latestClosed = new Map<string, string>()
    for (const message of visibleMessages) {
      const event = message.systemEvent
      if (event?.serviceSessionId && event.type === ConversationSystemEventType.ConversationSystemEventServiceSessionClosed) {
        latestClosed.set(event.serviceSessionId, message.id)
      }
      if (event?.serviceSessionId && event.type === ConversationSystemEventType.ConversationSystemEventServiceSessionReopened) {
        latestClosed.delete(event.serviceSessionId)
      }
    }
    for (const id of latestClosed.values()) summaryEventIDs.add(id)
  }

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
        conversationType === ConversationType.ConversationTypeChannel),
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

  const dateFormatters = useMemo(
    () => createTimelineDateFormatters(i18n.resolvedLanguage, timeZone),
    [i18n.resolvedLanguage, timeZone],
  )

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

  const pageLoadDisabled = Boolean(timeline.loadingDirection) || timeline.switching
  return (
    <div className="relative min-h-0 flex-1 bg-background">
      <ScrollArea
        ref={scrollRootRef}
        // 将 Radix Viewport 的内联 table 布局覆盖为块级布局。
        className="h-full min-h-0 bg-background [&>[data-slot=scroll-area-viewport]>div]:!flex [&>[data-slot=scroll-area-viewport]>div]:!min-h-full [&>[data-slot=scroll-area-viewport]>div]:!flex-col"
      >
        <div className="flex w-full flex-1 flex-col px-3.5 pb-2.5 md:px-5">
          {currentPage?.hasEarlier || timeline.pageError === "before" ? (
            <TimelinePageLoader
              direction="before"
              canLoad={Boolean(currentPage?.hasEarlier)}
              loading={timeline.loadingDirection === "before"}
              failed={timeline.pageError === "before"}
              disabled={pageLoadDisabled}
              onLoad={() => void loadPage("before")}
            />
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
              return (
                <TimelineMessageRow
                  key={message.id}
                  message={message}
                  previous={visibleMessages[index - 1]}
                  next={visibleMessages[index + 1]}
                  conversationID={conversationID}
                  conversationType={conversationType}
                  currentUser={currentUser}
                  formatters={dateFormatters}
                  highlighted={location.highlightedID === message.id}
                  summaryEvent={summaryEventIDs.has(message.id)}
                  customerDeliveries={customerDeliveries}
                  delivery={message.persistedMessageID ? deliveriesByMessage.get(message.persistedMessageID) : undefined}
                  deliveriesFailed={Boolean(deliveries.error)}
                  onRefreshDeliveries={() => void deliveries.refresh()}
                  sendingText={sendingText}
                  retryFailedMessageDisabled={retryFailedMessageDisabled}
                  onRetryFailedMessage={onRetryFailedMessage}
                  onReplyMessage={onReplyMessage}
                  noteReplyEnabled={noteReplyEnabled}
                  customerReplyUnavailable={customerReplyUnavailable}
                  replyVisibility={replyVisibility}
                  onFollowReference={followReference}
                  onToggleProcess={viewport.stopFollowing}
                  renderCodeBlock={onApplyReply ? renderCustomerReply : undefined}
                />
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
                incoming={conversationType !== ConversationType.ConversationTypeChannel}
                onToggle={viewport.stopFollowing}
              />
            ))
            : null}
          {timeline.mode === "latest" && !currentPage?.hasLater ? (
            <AgentQueueState
              agents={currentPage?.pendingAgents ?? []}
              copilot={conversationType === ConversationType.ConversationTypeCopilot}
              incoming={conversationType !== ConversationType.ConversationTypeChannel}
            />
          ) : null}
          {currentPage?.hasLater && timeline.mode === "anchor" ? (
            <TimelinePageLoader
              direction="after"
              canLoad
              loading={timeline.loadingDirection === "after"}
              failed={timeline.pageError === "after"}
              disabled={pageLoadDisabled}
              onLoad={() => void loadPage("after")}
            />
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
