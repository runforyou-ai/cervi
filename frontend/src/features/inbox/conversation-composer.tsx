/** 提交成员可回复会话的文本消息。 */
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type RefObject,
} from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { ArrowUpIcon, LoaderCircleIcon, MicIcon, StickyNoteIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ConversationType,
  ChannelType,
  MessageVisibility,
  isApiError,
  type ConversationMessageData,
  type CustomerInboxConversationData,
  type ConversationMessageReference,
  type DirectTextMessageInput,
  type GroupParticipant,
  type InboxConversation,
  type MemberOption,
} from "@/api"
import { IconTooltip } from "@/components/icon-tooltip"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import {
  createConversationComposerSchema,
  type ConversationComposerValues,
} from "@/features/inbox/conversation-composer-schema"
import {
  mentionTokenPattern,
  reconcileMentionAllToken,
} from "@/lib/mention-token"
import {
  conversationSendingIndicatorDelay,
  type MentionTarget,
  type OutgoingConversationDraft,
} from "@/features/inbox/outgoing-message-store"
import { CustomerReplyAssistant } from "@/features/inbox/customer-reply-assistant"
import { composerToolClass } from "@/features/inbox/composer-tool"
import { useConversationTypingReport } from "@/features/inbox/use-conversation-typing"
import { resolveAppPlatform } from "@/platform/app-platform"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

import { resizeComposerInput, useFocusInputOnTyping } from "./composer-input"
import { sendComposerTextMessage } from "./composer-send"
import {
  ComposerAttachmentTool,
  ComposerEmojiPicker,
  ComposerMentionOverlay,
  ComposerReplyPreview,
} from "./conversation-composer-parts"
import { useComposerMentions } from "./use-composer-mentions"
import { useVisibilityDrafts } from "./use-visibility-drafts"

/** 读取和替换回复输入框草稿的入口。 */
export type ComposerDraftBridge = {
  read: () => string
  replace: (body: string) => void
}

/** 客户会话所在渠道的附件与输入状态能力。 */
export type CustomerChannelCapabilities = Pick<
  CustomerInboxConversationData["customer"],
  "attachmentSupported" | "attachmentByteLimit" | "attachmentCaptionLimit" | "channelType"
>

/** 展示并提交成员会话文本编辑区。 */
export function ConversationComposer({
  conversationID,
  conversationType,
  submitOnEnter = false,
  refocusAfterSubmit = false,
  disabledReason: replyDisabledReason = null,
  visibility = MessageVisibility.MessageVisibilityCustomerVisible,
  onVisibilityChange,
  retryFailedMessage = false,
  retryDraft = null,
  replyTo = null,
  groupParticipants,
  noteMentionMembers,
  currentIdentityID = "",
  onRetryDraftHandled,
  onReplyToChange,
  onSending,
  onBeforeSend,
  onSent,
  onFailed,
  onSucceeded,
  sendIndividualMessage,
  attachmentTargetIdentityID,
  attachmentAgentDraft,
  customerChannel = null,
  onAttachmentConversationCreated,
  draftBridgeRef,
}: {
  attachmentTargetIdentityID?: string
  attachmentAgentDraft?: { conversationID: string; agentIdentityID: string; customerConversationID?: string }
  customerChannel?: CustomerChannelCapabilities | null
  onAttachmentConversationCreated?: (conversation: InboxConversation | null, conversationID: string) => void
  draftBridgeRef?: RefObject<ComposerDraftBridge | null>
  conversationID: string
  conversationType: ConversationType
  submitOnEnter?: boolean
  refocusAfterSubmit?: boolean
  disabledReason?: string | null
  visibility?: MessageVisibility
  onVisibilityChange?: (visibility: MessageVisibility) => void
  retryFailedMessage?: boolean
  retryDraft?: OutgoingConversationDraft | null
  replyTo?: ConversationMessageReference | null
  groupParticipants?: GroupParticipant[]
  noteMentionMembers?: MemberOption[]
  currentIdentityID?: string
  onRetryDraftHandled?: () => void
  onReplyToChange?: (message: ConversationMessageReference | null) => void
  onBeforeSend?: () => Promise<boolean>
  onSending: (message: OutgoingConversationDraft) => void
  onSent: (clientMessageID: string, message: ConversationMessageData) => void
  onFailed: (clientMessageID: string) => void
  onSucceeded: () => void
  sendIndividualMessage?: (
    input: DirectTextMessageInput,
  ) => Promise<ConversationMessageData>
}) {
  // 渠道能力缺省时按不支持附件和输入状态处理，附件说明默认上限 4000 字。
  const customerAttachmentSupported = Boolean(customerChannel?.attachmentSupported)
  const customerTypingSupported =
    customerChannel?.channelType === ChannelType.ChannelTypeWebsite
  const customerAttachmentByteLimit = customerChannel?.attachmentByteLimit ?? 0
  const customerAttachmentCaptionLimit = customerChannel?.attachmentCaptionLimit ?? 4000
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const aliveRef = useRef(true)
  const schema = useMemo(
    () =>
      createConversationComposerSchema({
        bodyTooLong: t("messageBodyTooLong"),
      }),
    [t],
  )
  const form = useForm<ConversationComposerValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { body: "" },
  })
  const inputID = `conversation-reply-${conversationID}`
  const inputRef = useRef<HTMLTextAreaElement | null>(null)
  const retryRef = useRef<OutgoingConversationDraft | null>(null)
  const refocusPendingRef = useRef(false)
  const replyToRef = useRef(replyTo)
  replyToRef.current = replyTo
  const visibilityRef = useRef(visibility)
  visibilityRef.current = visibility
  // 移动端的提及候选和取消引用使用触屏尺寸。
  const mobile = resolveAppPlatform() === "mobile"
  const { isSubmitting } = form.formState
  const bodyValue = form.watch("body")
  const isBodyEmpty = !bodyValue.trim()
  const bodyField = form.register("body")
  const internalNote =
    visibility === MessageVisibility.MessageVisibilityInternalOnly
  // 内部备注不经渠道投递，不受对客发送资格限制。
  const disabledReason = internalNote ? null : replyDisabledReason
  const groupConversation = conversationType === ConversationType.ConversationTypeGroup
  const customerConversation = conversationType === ConversationType.ConversationTypeCustomer
  // 单聊与群聊向其他成员上报本人正在输入；客户会话只有网站渠道在对客回复时向访客上报。
  const typingReport = useConversationTypingReport(
    conversationID,
    (groupConversation ||
      conversationType === ConversationType.ConversationTypeDirect ||
      (customerConversation && customerTypingSupported && !internalNote)) &&
      !disabledReason,
  )
  const {
    mentions,
    setMentions,
    mentionsRef,
    mentionAllToken,
    setMentionAllToken,
    mentionAll,
    mentionQuery,
    setMentionQuery,
    activeMentionIndex,
    mentionCandidates,
    noteMentionHint,
    updateMentionQuery,
    reconcileMentions,
    selectMention,
    handleMentionKeyDown,
  } = useComposerMentions({
    form,
    inputRef,
    typingReport,
    groupConversation,
    customerConversation,
    internalNote,
    groupParticipants,
    noteMentionMembers,
    currentIdentityID,
    noteSwitchAvailable: Boolean(onVisibilityChange),
  })
  // 对客草稿与内部备注草稿各自保留正文和提醒成员，切换页签时互不覆盖。
  const { draftsRef, focusAfterSwitchRef, switchVisibility, stashDraft } = useVisibilityDrafts({
    form,
    visibility,
    onVisibilityChange,
    inputRef,
    mentionsRef,
    setMentions,
    closeMentionQuery: () => setMentionQuery(null),
  })

  useEffect(() => {
    aliveRef.current = true
    resizeComposerInput(inputRef.current)
    return () => {
      aliveRef.current = false
    }
  }, [])

  useEffect(() => {
    if (!retryFailedMessage || !retryDraft) return
    onRetryDraftHandled?.()
    if (isSubmitting) return
    retryRef.current = retryDraft
    // 失败消息回到发送时的可见范围，当前页签属于另一种可见范围时先存入对应草稿。
    if (retryDraft.visibility !== visibility) {
      stashDraft(retryDraft.visibility, retryDraft.body, retryDraft.mentions)
      return
    }
    form.setValue("body", retryDraft.body, { shouldDirty: true })
    setMentions(retryDraft.mentions)
    setMentionAllToken(retryDraft.mentionAllToken)
    onReplyToChange?.(retryDraft.replyTo)
    resizeComposerInput(inputRef.current)
    form.setFocus("body")
  }, [
    form,
    isSubmitting,
    onRetryDraftHandled,
    onReplyToChange,
    retryDraft,
    retryFailedMessage,
    stashDraft,
    visibility,
  ])

  /** 用选中的表情替换正文当前选区，返回插入内容之后的光标位置。 */
  function insertEmoji(emoji: string) {
    const input = inputRef.current
    if (!input) return null
    const body = form.getValues("body")
    const start = input.selectionStart ?? body.length
    const end = input.selectionEnd ?? start
    const nextBody = `${body.slice(0, start)}${emoji}${body.slice(end)}`
    const nextCaret = start + emoji.length
    setMentionAllToken((current) =>
      reconcileMentionAllToken(current, body, nextBody, nextCaret),
    )
    form.setValue("body", nextBody, { shouldDirty: true })
    typingReport.input(nextBody)
    reconcileMentions(nextBody)
    resizeComposerInput(input)
    return nextCaret
  }

  useEffect(() => {
    if (isSubmitting || !refocusPendingRef.current) return
    refocusPendingRef.current = false
    form.setFocus("body")
  }, [form, isSubmitting])

  useEffect(() => {
    if (replyTo && !isSubmitting) {
      form.setFocus("body")
    }
  }, [form, isSubmitting, replyTo])

  useFocusInputOnTyping(inputRef, !disabledReason && !isSubmitting)

  /** 按会话类型发送当前成员文本消息。 */
  async function send(values: ConversationComposerValues) {
    if (disabledReason) return
    const body = values.body.trim()
    if (!body || replyTo?.deleted) return
    if (onBeforeSend && !(await onBeforeSend())) return
    if (!aliveRef.current) return
    // 草稿正文去掉首部空白后，同步调整结构化标记的位置。
    const rawBody = form.getValues("body")
    const leadingWhitespace = rawBody.length - rawBody.trimStart().length
    const draftMentionAllToken = mentionAllToken
      ? { ...mentionAllToken, start: mentionAllToken.start - leadingWhitespace }
      : null
    // 提醒顺序决定被点名 AI 员工的发言先后，按正文中标记出现的位置排序。
    const mentionPositions = new Map(
      mentions.map((mention) => {
        const match = body.match(new RegExp(mentionTokenPattern([mention.displayName]), "u"))
        return [mention.identityID, match?.index ?? Number.MAX_SAFE_INTEGER] as const
      }),
    )
    const orderedMentions = [...mentions].sort(
      (left, right) =>
        (mentionPositions.get(left.identityID) ?? 0) - (mentionPositions.get(right.identityID) ?? 0),
    )
    const mentionKey = (targets: MentionTarget[]) =>
      targets.map((mention) => mention.identityID).join("\u0000")
    const retry =
      retryFailedMessage &&
      retryRef.current?.body === body &&
      retryRef.current.visibility === visibility &&
      retryRef.current.replyTo?.id === replyTo?.id &&
      retryRef.current.mentionAll === mentionAll &&
      mentionKey(retryRef.current.mentions) === mentionKey(orderedMentions)
        ? retryRef.current
        : null
    const draft = {
      clientMessageID: retry?.clientMessageID ?? window.crypto.randomUUID(),
      visibility,
      body,
      originatedAt: retry?.originatedAt ?? new Date().toISOString(),
      replyTo: replyTo,
      mentions: orderedMentions,
      mentionAll,
      mentionAllToken: draftMentionAllToken,
    }
    retryRef.current = null
    onSending(draft)
    const { clientMessageID } = draft
    form.resetField("body")
    typingReport.stop()
    // 提醒状态随正文一起清空，发送失败时按草稿所属可见范围恢复。
    setMentions([])
    setMentionAllToken(null)
    resizeComposerInput(inputRef.current)
    try {
      const message = await sendComposerTextMessage({
        conversationType,
        conversationID,
        clientMessageID,
        body,
        replyToMessageID: replyTo?.id ?? "",
        visibility,
        mentions: orderedMentions,
        mentionAll,
        sendIndividualMessage,
      })
      onSucceeded()
      // 按发送逻辑编号写入发送结果。
      onSent(clientMessageID, message)
      if (!aliveRef.current) return
      setMentionQuery(null)
      // 发送期间切换了页签时，引用目标属于另一种可见范围，保持原样。
      if (visibilityRef.current === draft.visibility && replyToRef.current?.id === replyTo?.id) {
        onReplyToChange?.(null)
      }
      refocusPendingRef.current = refocusAfterSubmit
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("发送成员会话消息失败", {
        conversationId: conversationID,
        error,
      })
      onFailed(clientMessageID)
      if (!aliveRef.current) return
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "replyToMessageId",
              "mentionSubjectIds",
              "mentionIdentityIds",
              "body",
            ])
          : t("messageSendError"),
      )
      if (retryFailedMessage) {
        retryRef.current = draft
        // 发送期间切换了页签时，失败正文回到发送时的可见范围。
        if (draft.visibility !== visibilityRef.current) {
          stashDraft(draft.visibility, body, draft.mentions)
        } else {
          form.setValue("body", body, { shouldDirty: true })
          setMentions(draft.mentions)
          setMentionAllToken(draft.mentionAllToken)
          resizeComposerInput(inputRef.current)
        }
      }
      refocusPendingRef.current = refocusAfterSubmit
    }
  }

  /** 用 AI 生成的回复替换当前对客草稿并聚焦输入框。 */
  const applyReplySuggestion = useCallback((reply: string) => {
    // 候选回复始终填入对客草稿，内部备注模式下先切回对客页签。
    if (visibility === MessageVisibility.MessageVisibilityInternalOnly) {
      draftsRef.current[MessageVisibility.MessageVisibilityCustomerVisible] = reply
      focusAfterSwitchRef.current = true
      onVisibilityChange?.(MessageVisibility.MessageVisibilityCustomerVisible)
      return
    }
    form.setValue("body", reply, { shouldDirty: true })
    window.requestAnimationFrame(() => {
      resizeComposerInput(inputRef.current)
      form.setFocus("body")
    })
  }, [form, onVisibilityChange, visibility])

  useEffect(() => {
    if (!draftBridgeRef) return
    // 向 AI 助手提供读取和替换对客草稿的入口。
    draftBridgeRef.current = {
      read: () =>
        internalNote
          ? (draftsRef.current[MessageVisibility.MessageVisibilityCustomerVisible] ?? "")
          : form.getValues("body"),
      replace: applyReplySuggestion,
    }
    return () => {
      draftBridgeRef.current = null
    }
  }, [applyReplySuggestion, draftBridgeRef, form, internalNote])

  /** 在桌面键盘上提交消息，并保留 Shift+Enter 换行。 */
  function submitFromKeyboard(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (disabledReason) return
    const composing =
      event.keyCode === 229 || event.nativeEvent.isComposing
    if (handleMentionKeyDown(event, composing)) return
    // 提示可见时 Enter 切到内部备注，Escape 关闭提示，均不发送对客消息。
    if (!composing && noteMentionHint && (event.key === "Escape" || (event.key === "Enter" && !event.shiftKey))) {
      event.preventDefault()
      if (event.key === "Enter") switchVisibility(MessageVisibility.MessageVisibilityInternalOnly)
      else setMentionQuery(null)
      return
    }
    if (
      !submitOnEnter ||
      event.key !== "Enter" ||
      event.shiftKey ||
      composing
    ) {
      return
    }
    event.preventDefault()
    if (!form.formState.isSubmitting && !isBodyEmpty) {
      void form.handleSubmit(send)()
    }
  }

  const [showSubmitting, setShowSubmitting] = useState(false)
  // 客户会话在输入区工具栏提供 AI 写回复入口，不可对客发送时保留显示并禁用。
  const replyAssistant =
    conversationType === ConversationType.ConversationTypeCustomer &&
    conversationID &&
    !internalNote ? (
      <CustomerReplyAssistant
        mobile={mobile}
        conversationID={conversationID}
        currentIdentityID={currentIdentityID}
        draft={bodyValue}
        replyToMessageID={replyTo && !replyTo.deleted ? replyTo.id : ""}
        disabled={isSubmitting || Boolean(disabledReason)}
        onApply={applyReplySuggestion}
      />
    ) : null

  useEffect(() => {
    if (!isSubmitting) {
      setShowSubmitting(false)
      return
    }
    const timer = window.setTimeout(
      () => setShowSubmitting(true),
      conversationSendingIndicatorDelay,
    )
    return () => window.clearTimeout(timer)
  }, [isSubmitting])

  // 输入内容后语音入口换成发送按钮，并保持到本次发送结束。
  const showSend = !isBodyEmpty || isSubmitting
  const bodyInput = (
    <Textarea
      {...bodyField}
      ref={(input) => {
        bodyField.ref(input)
        inputRef.current = input
      }}
      id={inputID}
      disabled={isSubmitting}
      readOnly={Boolean(disabledReason)}
      rows={1}
      aria-label={t(internalNote ? "internalNoteLabel" : "replyLabel")}
      aria-describedby={disabledReason ? `${inputID}-reason` : undefined}
      aria-invalid={form.formState.errors.body ? true : undefined}
      className={cn(
        // 行高贴近字体自然行高，避免换行前后光标高度跳变；上下内边距之和保持 16px，下伸部留空由上多下少补偿。
        "max-h-[200px] min-h-10 min-w-0 flex-1 resize-none rounded-none border-0 bg-transparent px-0.5 pt-[11px] pb-[9px] leading-5 shadow-none focus-visible:ring-0 dark:bg-transparent",
        // 正文各端统一 16px，移动端聚焦时不缩放，与访客端一致。
        "md:text-[1rem]",
      )}
      onInput={(event) => {
        resizeComposerInput(event.currentTarget)
      }}
      onChange={(event) => {
        const previousBody = form.getValues("body")
        const { value, selectionStart } = event.currentTarget
        setMentionAllToken((current) =>
          reconcileMentionAllToken(
            current, previousBody, value, selectionStart,
          ),
        )
        bodyField.onChange(event)
        typingReport.input(event.currentTarget.value)
        reconcileMentions(event.currentTarget.value)
        updateMentionQuery(
          event.currentTarget.value,
          event.currentTarget.selectionStart,
        )
      }}
      onClick={(event) =>
        updateMentionQuery(
          event.currentTarget.value,
          event.currentTarget.selectionStart,
        )
      }
      onBlur={(event) => {
        bodyField.onBlur(event)
        setMentionQuery(null)
      }}
      onKeyDown={submitFromKeyboard}
    />
  )

  return (
    <form
      data-slot="conversation-composer"
      data-conversation-id={conversationID}
      className="shrink-0 bg-background"
      onSubmit={form.handleSubmit(send)}
      noValidate
    >
      <div className="relative">
        <ComposerMentionOverlay
          candidates={mentionCandidates}
          activeIndex={activeMentionIndex}
          groupConversation={groupConversation}
          showCandidates={!disabledReason && Boolean(mentionQuery) && mentionCandidates.length > 0}
          showNoteHint={!disabledReason && noteMentionHint}
          onSelect={selectMention}
          onSwitchToNote={() =>
            switchVisibility(MessageVisibility.MessageVisibilityInternalOnly)
          }
        />
        <div
          className={cn(
            internalNote
              ? "bg-note/60"
              : "bg-background",
          )}
        >
          {replyTo ? (
            <ComposerReplyPreview
              replyTo={replyTo}
              disabled={Boolean(disabledReason)}
              onCancel={() => onReplyToChange?.(null)}
            />
          ) : null}
          {/* 输入区整体铺底色，正文单独用白底并与上下边缘留出间距。 */}
          <div className="flex items-end gap-2 bg-foreground/[0.03] px-2 py-1">
            <div className="mb-1.5 flex items-end">
              {onVisibilityChange ? (
                <IconTooltip label={t("composerModeNote")}>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      aria-pressed={internalNote}
                      className={cn(
                        composerToolClass,
                        internalNote &&
                          "bg-note-active text-note-active-foreground hover:bg-note-active hover:text-note-active-foreground",
                      )}
                      aria-label={t("composerModeNote")}
                      onClick={() =>
                        switchVisibility(
                          internalNote
                            ? MessageVisibility.MessageVisibilityCustomerVisible
                            : MessageVisibility.MessageVisibilityInternalOnly,
                        )
                      }
                    >
                      <StickyNoteIcon />
                    </Button>
                </IconTooltip>
              ) : null}
              <ComposerAttachmentTool
                conversationID={conversationID}
                conversationType={conversationType}
                customerEnabled={customerAttachmentSupported && !internalNote}
                targetIdentityID={attachmentTargetIdentityID}
                agentDraft={attachmentAgentDraft}
                byteLimit={customerAttachmentByteLimit}
                captionLimit={customerAttachmentCaptionLimit}
                replyTo={replyTo}
                disabled={isSubmitting || Boolean(disabledReason)}
                onSent={() => onReplyToChange?.(null)}
                onBeforeSend={onBeforeSend}
                onCreated={onAttachmentConversationCreated}
              />
            </div>
            <div className="flex min-w-0 flex-1 items-end rounded-md bg-background px-2">
              {disabledReason ? (
                <p
                  id={`${inputID}-reason`}
                  className="min-w-0 flex-1 truncate py-[10px] text-xs leading-5 text-muted-foreground"
                >
                  {disabledReason}
                </p>
              ) : (
                bodyInput
              )}
            </div>
            <div className="mb-1.5 flex items-end">
              <ComposerEmojiPicker
                disabled={isSubmitting || Boolean(disabledReason)}
                inputRef={inputRef}
                onInsert={insertEmoji}
              />
              {replyAssistant}
              {showSend ? (
                <IconTooltip label={t(internalNote ? "internalNoteSave" : "messageSend")}>
                  <Button
                    type="submit"
                    size="icon-sm"
                    className="relative rounded-full after:absolute after:-inset-1 after:content-['']"
                    disabled={isSubmitting || Boolean(disabledReason) || isBodyEmpty || replyTo?.deleted}
                    aria-label={t(internalNote ? "internalNoteSave" : "messageSend")}
                  >
                    {showSubmitting ? <LoaderCircleIcon className="animate-spin" /> : <ArrowUpIcon />}
                  </Button>
                </IconTooltip>
              ) : (
                <IconTooltip label={t("voiceMessage")}>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className={composerToolClass}
                    disabled
                    aria-label={t("voiceMessage")}
                  >
                    <MicIcon />
                  </Button>
                </IconTooltip>
              )}
            </div>
          </div>
        </div>
      </div>
    </form>
  )
}
