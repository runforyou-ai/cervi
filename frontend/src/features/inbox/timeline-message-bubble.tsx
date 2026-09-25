/** 时间线中的消息气泡：发送者、头像、引用块、正文、时间与投递状态，以及回复和复制操作。 */
import { useRef, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  ChatSubjectKind,
  ConversationType,
  MessageType,
  MessageVisibility,
  type ConversationMessageReference,
  type CurrentUser,
  type CustomerMessageDelivery,
} from "@/api"
import { MessageMarkdown } from "@/components/message-markdown"
import { ProfileAvatar } from "@/components/profile-avatar"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import type { OutgoingConversationDraft } from "@/features/inbox/outgoing-message-store"
import { mentionTokenPattern } from "@/lib/mention-token"
import { messagePreview } from "@/lib/message-preview"
import { cn } from "@/lib/utils"
import { resolveAppPlatform } from "@/platform/app-platform"
import { openExternalURL } from "@/platform/external-navigation"

import { AgentProcess } from "./agent-process"
import { ConversationAttachment } from "./conversation-attachment"
import { CustomerDeliveryState } from "./customer-delivery-state"
import { MessageSendState } from "./message-send-state"
import type { TimelineDateFormatters } from "./timeline-grouping"
import { messageMentionNames, type TimelineMessage } from "./timeline-messages"
import { isAIIdentityType } from "@/lib/identity-type"
import { languageDisplayName } from "@/lib/languages"
import { useMessageTranslation, type MessageTranslationView } from "./message-translation"

/** 消息气泡依赖的会话上下文、投递状态与操作入口。 */
export type TimelineMessageBubbleContext = {
  conversationID: string
  conversationType: ConversationType
  currentUser: CurrentUser
  formatters: TimelineDateFormatters
  highlighted: boolean
  customerDeliveries: boolean
  delivery: CustomerMessageDelivery | undefined
  deliveriesFailed: boolean
  onRefreshDeliveries: () => void
  sendingText: boolean
  retryFailedMessageDisabled: boolean
  onRetryFailedMessage?: (message: OutgoingConversationDraft) => void
  onReplyMessage?: (
    message: ConversationMessageReference,
    visibility: MessageVisibility,
  ) => void
  noteReplyEnabled: boolean
  customerReplyUnavailable: boolean
  replyVisibility: MessageVisibility
  onFollowReference: (messageID: string) => Promise<void>
  onToggleProcess: () => void
  renderCodeBlock?: (language: string, code: string) => ReactNode | undefined
}

/** 在消息正文中强调结构化提醒。 */
function renderMessageBody(message: TimelineMessage, body: string) {
  const names = messageMentionNames(message)
  if (names.length === 0) return body
  const mentioned = new Set(names.map((name) => `@${name}`))
  const parts = body.split(
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

/** 气泡组件的属性：会话上下文加单条消息及其分组位置。 */
type TimelineMessageBubbleProps = TimelineMessageBubbleContext & {
  message: TimelineMessage
  date: Date
  spaced: boolean
  startsGroup: boolean
  endsGroup: boolean
}

/** 展示一条非系统事件消息的气泡及其右键菜单。 */
export function TimelineMessageBubble(props: TimelineMessageBubbleProps) {
  const {
    message,
    date,
    spaced,
    startsGroup,
    endsGroup,
    conversationType,
    currentUser,
    formatters,
    highlighted,
    onReplyMessage,
    noteReplyEnabled,
    customerReplyUnavailable,
    replyVisibility,
  } = props
  const { t } = useTranslation(["inbox", "common"])
  const rowRef = useRef<HTMLElement>(null)
  const currentIdentityID = currentUser.identityId
  // 右侧 AI 助手面板宽度有限，消息不展示头像，改在气泡上方标出发送者。
  const copilot = conversationType === ConversationType.ConversationTypeCopilot
  // 移动端气泡禁止文本选择，消息菜单项使用触屏尺寸，回复入口只通过长按菜单提供。
  const mobile = resolveAppPlatform() === "mobile"
  const menuItemClassName = "touch:min-h-11"
  const agentError = message.type === MessageType.MessageTypeAgentError
  const agentCancelled = message.type === MessageType.MessageTypeAgentCancelled
  const agentNotice = agentError || agentCancelled
  const incoming = message.local
    ? false
    : conversationType !== ConversationType.ConversationTypeChannel
      ? !message.sender ||
        message.sender.sourceId !== currentIdentityID
      : !message.sender ||
        message.sender.kind ===
          ChatSubjectKind.ChatSubjectKindContact
  const translation = useMessageTranslation(
    message,
    message.sender?.kind === ChatSubjectKind.ChatSubjectKindContact,
    rowRef,
  )
  const sentByCurrentIdentity =
    !message.local &&
    conversationType !==
      ConversationType.ConversationTypeChannel &&
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
  // 回复与复制使用气泡当前显示的正文，附件消息没有说明时取文件名。
  const referenceBody = translation.body || message.attachment?.name || ""
  const internalNote =
    message.visibility === MessageVisibility.MessageVisibilityInternal
  // 引用落入的输入模式：当前处于备注模式，或这条消息不能用于对客回复时，都写入内部备注。
  const quoteAsNote =
    noteReplyEnabled &&
    (replyVisibility === MessageVisibility.MessageVisibilityInternal ||
      customerReplyUnavailable ||
      !message.canReply)
  const quoteDisabled = quoteAsNote
    ? !message.canNoteReply
    : !message.canReply
  const quoteVisibility = quoteAsNote
    ? MessageVisibility.MessageVisibilityInternal
    : MessageVisibility.MessageVisibilityShared
  // 悬停回复按钮与右键菜单共用同一个引用入口。
  const quoteMessage = () =>
    onReplyMessage?.(
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
  // 文字气泡与附件气泡共用同一套方向配色和组尾圆角，内部备注使用区别于对客消息的常驻样式。
  const bubbleClassName = cn(
    internalNote
      ? "border border-dashed border-note-border bg-note text-foreground"
      : incoming || agentNotice
        ? "border bg-muted text-foreground shadow-xs"
        : "bg-accent text-accent-foreground",
    endsGroup && (incoming ? "rounded-bl-sm" : "rounded-br-sm"),
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

  return (
    <ContextMenu>
      <article
        ref={rowRef}
        data-message-id={message.local ? undefined : message.id}
        tabIndex={-1}
        className={cn(
          highlighted &&
            "message-location-highlight",
          "group/message-row flex items-start gap-2",
          spaced && (startsGroup ? "mt-3" : "mt-1"),
          incoming ? "justify-start" : "justify-end",
        )}
        aria-label={`${senderName} ${formatters.full.format(date)}`}
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
            <span className="max-w-full truncate text-xs font-medium text-note-foreground">
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
                fallback={isAIIdentityType(message.sender?.identityType)
                  ? "agent"
                  : "person"}
                className={cn(
                  "absolute bottom-0 size-7",
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
                    onClick={() => quoteMessage()}
                  >
                    {t("messageReply")}
                  </button>
                ) : null}
                <MessageBubbleContent
                  {...props}
                  body={translation.body}
                  translation={translation.view}
                  incoming={incoming}
                  internalNote={internalNote}
                  bubbleClassName={bubbleClassName}
                />
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
            onSelect={() => quoteMessage()}
          >
            {t("messageReply")}
          </ContextMenuItem>
        ) : null}
        {translation.view.status === "translated" ? (
          <ContextMenuItem className={menuItemClassName} onSelect={translation.view.toggle}>
            {translationToggleLabel(translation.view.showingOriginal, !incoming, t)}
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
  )
}

/** 展示气泡内的引用块、Agent 过程、正文、时间与投递状态。 */
function MessageBubbleContent({
  message,
  body,
  translation,
  date,
  incoming,
  internalNote,
  bubbleClassName,
  conversationID,
  formatters,
  customerDeliveries,
  delivery,
  deliveriesFailed,
  onRefreshDeliveries,
  sendingText,
  retryFailedMessageDisabled,
  onRetryFailedMessage,
  onFollowReference,
  onToggleProcess,
  renderCodeBlock,
}: TimelineMessageBubbleProps & {
  body: string
  translation: MessageTranslationView
  incoming: boolean
  internalNote: boolean
  bubbleClassName: string
}) {
  const { t, i18n } = useTranslation(["inbox", "common"])
  const agentError = message.type === MessageType.MessageTypeAgentError
  const agentNotice = agentError || message.type === MessageType.MessageTypeAgentCancelled
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
  // 有文本发送中时禁用文本重试，附件重试由上传队列排队；内部备注的重试不受对客发送资格限制，仍与发送中的文本互斥。
  const messageRetryDisabled = internalNote ? sendingText : retryFailedMessageDisabled || sendingText
  // 成员发往外部渠道的文本与附件共用同一份投递状态。
  const renderDeliveryState = customerDeliveries && !agentNotice && !internalNote && (message.local || message.sender?.kind === ChatSubjectKind.ChatSubjectKindOrganizationIdentity) ? (className?: string) => (
    <CustomerDeliveryState
      className={className}
      conversationID={conversationID}
      delivery={delivery}
      loadingError={Boolean(message.persistedMessageID) && deliveriesFailed}
      onRefresh={onRefreshDeliveries}
      localFailed={!message.attachment && message.deliveryStatus === "failed"}
      onRetryLocal={failedDraft && onRetryFailedMessage ? () => onRetryFailedMessage(failedDraft) : undefined}
      retryLocalDisabled={messageRetryDisabled}
    />
  ) : undefined
  return (
    <div
      className={cn(
        "min-w-0 max-w-full text-sm break-words [overflow-wrap:anywhere]",
        !message.attachment && cn("rounded-2xl px-3 py-2", bubbleClassName),
      )}
    >
      {message.replyTo ? (
        <MessageReplyQuote
          replyTo={message.replyTo}
          // 内部备注与附件消息的引用块在对客气泡外，按所在背景取色。
          outside={incoming || internalNote || Boolean(message.attachment)}
          onFollow={onFollowReference}
        />
      ) : null}
      {message.agentProcess ? (
        <AgentProcess process={message.agentProcess} onPrimary={!incoming && !agentNotice} onToggle={onToggleProcess} />
      ) : null}
      {/* 时间跟随正文末行，正文按整行宽度排版。 */}
      <div
        className="cervi-message-body relative min-w-0 after:block after:clear-both after:content-['']"
        data-delivery={Boolean(renderDeliveryState || message.deliveryStatus) || undefined}
        data-translation={translation.status !== "none" || undefined}
      >
        {agentNotice ? (
          <span className={agentError ? "text-destructive" : "text-muted-foreground"}>{t(agentError ? "agentRunFailed" : "agentReplyStopped")}</span>
        ) : message.attachment ? (
          <ConversationAttachment retryDisabled={retryFailedMessageDisabled} body={body} attachment={message.attachment} conversationID={conversationID} messageID={message.persistedMessageID ?? message.id}
            originatedAt={message.originatedAt} timeLabel={formatters.clock.format(date)} timeTitle={formatters.full.format(date)} incoming={incoming} bubbleClassName={bubbleClassName} renderDeliveryState={renderDeliveryState} />
        ) : isAIIdentityType(message.sender?.identityType) ? (
          <div className="min-w-0">
            <MessageMarkdown
              locale={i18n.language}
              mentions={messageMentionNames(message)}
              onOpenLink={openExternalURL}
              renderCodeBlock={renderCodeBlock}
            >
              {body}
            </MessageMarkdown>
          </div>
        ) : (
          <span className="whitespace-pre-wrap">{renderMessageBody(message, body)}</span>
        )}
        {!message.attachment ? (
          <MessageTimeMeta
            message={message}
            translation={translation}
            outgoing={!incoming}
            date={date}
            formatters={formatters}
            muted={incoming || agentNotice || internalNote}
            renderDeliveryState={renderDeliveryState}
            onRetry={failedDraft && onRetryFailedMessage ? () => onRetryFailedMessage(failedDraft) : undefined}
            retryDisabled={messageRetryDisabled}
          />
        ) : null}
      </div>
    </div>
  )
}

/** 展示被回复消息的摘要，点击跳转到原消息。 */
function MessageReplyQuote({
  replyTo,
  outside,
  onFollow,
}: {
  replyTo: NonNullable<TimelineMessage["replyTo"]>
  outside: boolean
  onFollow: (messageID: string) => Promise<void>
}) {
  const { t } = useTranslation(["inbox", "common"])
  return (
    <button
      type="button"
      disabled={replyTo.deleted || !replyTo.id}
      onClick={() =>
        void onFollow(
          replyTo.id,
        )
      }
      className={cn(
        "mb-1.5 block w-full border-l-2 pl-2 text-left text-xs focus-visible:outline focus-visible:outline-2",
        outside
          ? "border-primary text-muted-foreground"
          : "border-accent-foreground/60 text-accent-foreground/75",
      )}
      aria-label={
        replyTo.deleted
          ? t("messageOriginalDeleted")
          : t("messageGoToOriginal")
      }
    >
      {replyTo.deleted ? (
        t("messageOriginalDeleted")
      ) : (
        <>
          <span className="block font-medium">
            {replyTo.sender?.displayName?.trim() || replyTo.externalSenderName ||
              t(replyTo.sender?.kind === ChatSubjectKind.ChatSubjectKindContact ? "anonymousVisitor" : "unknownSender")}
          </span>
          <span className="line-clamp-2 whitespace-pre-wrap">
            {replyTo.body ? messagePreview(replyTo.body, replyTo.sender?.identityType) : t("messageOriginalUnavailable")}
          </span>
        </>
      )}
    </button>
  )
}

/** 在文本气泡正文末行右侧展示发送时间、投递状态与失败重试。 */
function MessageTimeMeta({
  message,
  translation,
  outgoing,
  date,
  formatters,
  muted,
  renderDeliveryState,
  onRetry,
  retryDisabled,
}: {
  message: TimelineMessage
  translation: MessageTranslationView
  outgoing: boolean
  date: Date
  formatters: TimelineDateFormatters
  muted: boolean
  renderDeliveryState: ((className?: string) => ReactNode) | undefined
  onRetry: (() => void) | undefined
  retryDisabled: boolean
}) {
  const { t } = useTranslation(["inbox", "common"])
  return (
    <div
      className={cn(
        "cervi-message-time float-right ml-2 inline-flex translate-y-0.5 items-center gap-1 whitespace-nowrap text-[10px]",
        muted
          ? "text-muted-foreground"
          : "text-accent-foreground/75",
      )}
    >
      <MessageTranslationLabel translation={translation} outgoing={outgoing} />
      <time
        dateTime={message.originatedAt}
        title={formatters.full.format(date)}
      >
        {formatters.clock.format(date)}
      </time>
      {renderDeliveryState ? renderDeliveryState()
      : message.deliveryStatus ? (
        <div className="inline-flex items-center gap-1.5 text-xs">
          <MessageSendState
            state={message.deliveryStatus === "failed" ? "attention" : "sending"}
            detail={message.deliveryStatus === "failed" ? t("messageSendError") : undefined}
          />
          {onRetry ? (
            <button
              type="button"
              className="underline-offset-2 hover:underline disabled:cursor-not-allowed disabled:opacity-50 disabled:no-underline"
              disabled={retryDisabled}
              onClick={onRetry}
            >
              {t("messageRetry")}
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

/** 在时间前展示译文来源并切换原文，翻译中与翻译失败时给出状态和重试。 */
function MessageTranslationLabel({ translation, outgoing }: { translation: MessageTranslationView; outgoing: boolean }) {
  const { t, i18n } = useTranslation("inbox")
  if (translation.status === "none") return null
  if (translation.status === "pending") return <span>{t("translationPending")}</span>
  if (translation.status === "failed") {
    return (
      <button type="button" className="underline-offset-2 hover:underline" onClick={translation.retry}>
        {t("translationFailedRetry")}
      </button>
    )
  }
  const language = languageDisplayName(translation.language, i18n.language)
  const action = translationToggleLabel(translation.showingOriginal, outgoing, t)
  return (
    <button
      type="button"
      className="underline-offset-2 hover:underline"
      aria-label={action}
      title={action}
      onClick={translation.toggle}
    >
      {translation.showingOriginal
        ? t(outgoing ? "translationCustomerReceived" : "translationOriginal")
        : t(outgoing ? "translationSentAs" : "translationFrom", { language })}
    </button>
  )
}

/** 返回切换译文的操作名：客户消息的正文是原文，客服与 AI 发出消息的正文是客户收到的内容。 */
function translationToggleLabel(showingOriginal: boolean, outgoing: boolean, t: (key: "translationShowTranslated" | "translationShowOriginal" | "translationShowCustomerReceived") => string) {
  if (showingOriginal) return t("translationShowTranslated")
  return t(outgoing ? "translationShowCustomerReceived" : "translationShowOriginal")
}
