/** 提交成员可回复会话的文本消息。 */
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon, PaperclipIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ConversationType,
  isApiError,
  sendCustomerTextMessage,
  sendDirectTextMessage,
  sendGroupTextMessage,
  type ConversationMessageData,
  type ConversationMessageReference,
  type GroupParticipant,
} from "@/api"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import {
  createConversationComposerSchema,
  type ConversationComposerValues,
} from "@/features/inbox/conversation-composer-schema"
import { mentionTokenPattern } from "@/features/inbox/mention-token"
import {
  conversationSendingIndicatorDelay,
  type OutgoingConversationDraft,
} from "@/features/inbox/use-outgoing-conversation-messages"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const conversationComposerMaxHeight = 200
const conversationComposerMinHeight = 80
const conversationComposerKeyboardResizeStep = 16

/** 统计正文中仍然存在的完整 @ 姓名标记。 */
function countMentionTokens(body: string, displayName: string) {
  return Array.from(
    body.matchAll(new RegExp(mentionTokenPattern([displayName]), "gu")),
  ).length
}

/** 根据文本内容和手动高度调整消息输入框。 */
function resizeComposerInput(
  input: HTMLTextAreaElement | null,
  manualHeight: number | null,
) {
  if (!input) return
  input.style.height = "auto"
  const contentHeight = Math.min(
    input.scrollHeight,
    conversationComposerMaxHeight,
  )
  input.style.height = `${Math.max(contentHeight, manualHeight ?? 0)}px`
  const renderedHeight = input.getBoundingClientRect().height
  input.style.overflowY = input.scrollHeight > renderedHeight ? "auto" : "hidden"
}

/** 展示并提交成员会话文本编辑区。 */
export function ConversationComposer({
  conversationID,
  conversationType,
  submitOnEnter = false,
  refocusAfterSubmit = false,
  retryFailedMessage = false,
  retryDraft = null,
  replyTo = null,
  groupParticipants,
  currentIdentityID = "",
  onRetryDraftHandled,
  onReplyToChange,
  onSending,
  onSent,
  onFailed,
  onSucceeded,
}: {
  conversationID: string
  conversationType: ConversationType
  submitOnEnter?: boolean
  refocusAfterSubmit?: boolean
  retryFailedMessage?: boolean
  retryDraft?: OutgoingConversationDraft | null
  replyTo?: ConversationMessageReference | null
  groupParticipants?: GroupParticipant[]
  currentIdentityID?: string
  onRetryDraftHandled?: () => void
  onReplyToChange?: (message: ConversationMessageReference | null) => void
  onSending: (message: OutgoingConversationDraft) => void
  onSent: (clientMessageID: string, message: ConversationMessageData) => void
  onFailed: (clientMessageID: string) => void
  onSucceeded: () => void
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const aliveRef = useRef(true)
  const schema = useMemo(
    () =>
      createConversationComposerSchema({
        bodyRequired: t("messageBodyRequired"),
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
  const manualInputHeightRef = useRef<number | null>(null)
  const resizeStartRef = useRef<{
    pointerY: number
    inputHeight: number
  } | null>(null)
  const retryRef = useRef<OutgoingConversationDraft | null>(null)
  const refocusPendingRef = useRef(false)
  const replyToRef = useRef(replyTo)
  replyToRef.current = replyTo
  const [mentionSubjectIDs, setMentionSubjectIDs] = useState<string[]>([])
  const [mentionQuery, setMentionQuery] = useState<{
    start: number
    value: string
  } | null>(null)
  const [activeMentionIndex, setActiveMentionIndex] = useState(0)
  const { isSubmitting } = form.formState
  const bodyField = form.register("body")

  useEffect(() => {
    aliveRef.current = true
    resizeComposerInput(inputRef.current, manualInputHeightRef.current)
    return () => {
      aliveRef.current = false
    }
  }, [])

  useEffect(() => {
    if (!retryFailedMessage || !retryDraft) return
    onRetryDraftHandled?.()
    if (isSubmitting) return
    retryRef.current = retryDraft
    form.setValue("body", retryDraft.body, { shouldDirty: true })
    setMentionSubjectIDs(retryDraft.mentionSubjectIDs)
    onReplyToChange?.(retryDraft.replyTo)
    resizeComposerInput(inputRef.current, manualInputHeightRef.current)
    form.setFocus("body")
  }, [
    form,
    isSubmitting,
    onRetryDraftHandled,
    onReplyToChange,
    retryDraft,
    retryFailedMessage,
  ])

  const mentionCandidates = useMemo(() => {
    if (!mentionQuery || !groupParticipants) return []
    const query = mentionQuery.value.toLocaleLowerCase()
    return groupParticipants
      .filter(
        (participant) =>
          participant.identityId !== currentIdentityID &&
          !mentionSubjectIDs.includes(participant.chatSubjectId) &&
          participant.displayName.toLocaleLowerCase().includes(query),
      )
      .slice(0, 8)
  }, [
    currentIdentityID,
    groupParticipants,
    mentionQuery,
    mentionSubjectIDs,
  ])

  /** 根据光标前文本更新 @ 候选查询。 */
  function updateMentionQuery(value: string, selectionStart: number | null) {
    if (
      conversationType !== ConversationType.ConversationTypeGroup ||
      selectionStart === null
    ) {
      setMentionQuery(null)
      return
    }
    const beforeCaret = value.slice(0, selectionStart)
    const match = beforeCaret.match(/(?:^|\s)@([^\s@]*)$/)
    if (!match) {
      setMentionQuery(null)
      return
    }
    const markerOffset = match[0].lastIndexOf("@")
    setMentionQuery({
      start: beforeCaret.length - match[0].length + markerOffset,
      value: match[1],
    })
    setActiveMentionIndex(0)
  }

  /** 删除正文中已经不存在的结构化提醒目标。 */
  function reconcileMentionSubjects(value: string) {
    if (!groupParticipants) return
    setMentionSubjectIDs((current) => {
      const remainingByName = new Map<string, number>()
      return current.filter((subjectID) => {
        const participant = groupParticipants.find(
          (candidate) => candidate.chatSubjectId === subjectID,
        )
        if (!participant) return false
        const remaining =
          remainingByName.get(participant.displayName) ??
          countMentionTokens(value, participant.displayName)
        remainingByName.set(participant.displayName, Math.max(remaining - 1, 0))
        return remaining > 0
      })
    })
  }

  /** 插入选中的群成员并记录稳定聊天主体。 */
  function selectMention(participant: GroupParticipant) {
    const query = mentionQuery
    const input = inputRef.current
    if (!query || !input) return
    const body = form.getValues("body")
    const caret = input.selectionStart ?? body.length
    const token = `@${participant.displayName} `
    const nextBody = `${body.slice(0, query.start)}${token}${body.slice(caret)}`
    const nextCaret = query.start + token.length
    form.setValue("body", nextBody, { shouldDirty: true })
    setMentionSubjectIDs((current) =>
      current.includes(participant.chatSubjectId)
        ? current
        : [...current, participant.chatSubjectId],
    )
    setMentionQuery(null)
    window.requestAnimationFrame(() => {
      input.focus()
      input.setSelectionRange(nextCaret, nextCaret)
      resizeComposerInput(input, manualInputHeightRef.current)
    })
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

  /** 按会话类型发送当前成员文本消息。 */
  async function send(values: ConversationComposerValues) {
    const body = values.body.trim()
    const normalizedMentionSubjectIDs = [...mentionSubjectIDs].sort()
    const activeReplyTo =
      conversationType === ConversationType.ConversationTypeGroup
        ? replyTo
        : null
    const retry =
      retryFailedMessage &&
      retryRef.current?.body === body &&
      retryRef.current.replyTo?.id === activeReplyTo?.id &&
      retryRef.current.mentionSubjectIDs.join("\u0000") ===
        normalizedMentionSubjectIDs.join("\u0000")
        ? retryRef.current
        : null
    const draft = {
      clientMessageID: retry?.clientMessageID ?? window.crypto.randomUUID(),
      body,
      originatedAt: retry?.originatedAt ?? new Date().toISOString(),
      replyTo: activeReplyTo,
      mentionSubjectIDs: normalizedMentionSubjectIDs,
    }
    retryRef.current = null
    onSending(draft)
    const { clientMessageID } = draft
    const input = inputRef.current
    if (input) setManualInputHeight(input.getBoundingClientRect().height)
    form.resetField("body")
    resizeComposerInput(inputRef.current, manualInputHeightRef.current)
    try {
      const messageInput = { clientMessageId: clientMessageID, body }
      let message: ConversationMessageData
      switch (conversationType) {
        case ConversationType.ConversationTypeDirect:
          message = await sendDirectTextMessage(conversationID, messageInput)
          break
        case ConversationType.ConversationTypeGroup:
          message = await sendGroupTextMessage(conversationID, {
            ...messageInput,
            replyToMessageId: activeReplyTo?.id ?? "",
            mentionSubjectIds: normalizedMentionSubjectIDs,
          })
          break
        case ConversationType.ConversationTypeCustomer:
          message = await sendCustomerTextMessage(conversationID, messageInput)
          break
        default:
          throw new Error("不支持的会话类型")
      }
      onSucceeded()
      if (!aliveRef.current) return
      setMentionSubjectIDs([])
      setMentionQuery(null)
      if (replyToRef.current?.id === activeReplyTo?.id) {
        onReplyToChange?.(null)
      }
      refocusPendingRef.current = refocusAfterSubmit
      onSent(clientMessageID, message)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (!aliveRef.current) return
      console.warn("发送成员会话消息失败", {
        conversationId: conversationID,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "replyToMessageId",
              "mentionSubjectIds",
              "body",
            ])
          : t("messageSendError"),
      )
      if (retryFailedMessage) {
        retryRef.current = draft
        form.setValue("body", body, { shouldDirty: true })
        resizeComposerInput(inputRef.current, manualInputHeightRef.current)
      }
      refocusPendingRef.current = refocusAfterSubmit
      onFailed(clientMessageID)
    }
  }

  /** 在桌面键盘上提交消息，并保留 Shift+Enter 换行。 */
  function submitFromKeyboard(event: KeyboardEvent<HTMLTextAreaElement>) {
    const composing =
      event.keyCode === 229 || event.nativeEvent.isComposing
    if (!composing && mentionQuery && mentionCandidates.length > 0) {
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault()
        const direction = event.key === "ArrowDown" ? 1 : -1
        setActiveMentionIndex((current) =>
          (current + direction + mentionCandidates.length) %
          mentionCandidates.length,
        )
        return
      }
      if (event.key === "Enter") {
        event.preventDefault()
        selectMention(
          mentionCandidates[
            Math.min(activeMentionIndex, mentionCandidates.length - 1)
          ],
        )
        return
      }
      if (event.key === "Escape") {
        event.preventDefault()
        setMentionQuery(null)
        return
      }
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
    if (!form.formState.isSubmitting) {
      void form.handleSubmit(send)()
    }
  }

  /** 应用用户选择的消息输入框高度。 */
  function setManualInputHeight(height: number) {
    const input = inputRef.current
    if (!input) return
    const nextHeight = Math.min(
      conversationComposerMaxHeight,
      Math.max(conversationComposerMinHeight, height),
    )
    manualInputHeightRef.current = nextHeight
    resizeComposerInput(input, nextHeight)
  }

  /** 开始拖动消息输入框。 */
  function startInputResize(event: ReactPointerEvent<HTMLButtonElement>) {
    const input = inputRef.current
    if (!input) return
    event.preventDefault()
    event.currentTarget.setPointerCapture(event.pointerId)
    resizeStartRef.current = {
      pointerY: event.clientY,
      inputHeight: input.getBoundingClientRect().height,
    }
  }

  /** 按指针位置调整消息输入框高度。 */
  function resizeInput(event: ReactPointerEvent<HTMLButtonElement>) {
    const start = resizeStartRef.current
    if (!start || !event.currentTarget.hasPointerCapture(event.pointerId)) return
    setManualInputHeight(start.inputHeight + start.pointerY - event.clientY)
  }

  /** 结束拖动消息输入框。 */
  function stopInputResize(event: ReactPointerEvent<HTMLButtonElement>) {
    resizeStartRef.current = null
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
  }

  /** 使用方向键调整消息输入框高度。 */
  function resizeInputFromKeyboard(event: KeyboardEvent<HTMLButtonElement>) {
    if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return
    const input = inputRef.current
    if (!input) return
    event.preventDefault()
    const direction = event.key === "ArrowUp" ? 1 : -1
    setManualInputHeight(
      input.getBoundingClientRect().height +
        direction * conversationComposerKeyboardResizeStep,
    )
  }

  const [showSubmitting, setShowSubmitting] = useState(false)

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

  return (
    <form
      data-slot="conversation-composer"
      data-conversation-id={conversationID}
      className="shrink-0 bg-background p-3"
      onSubmit={form.handleSubmit(send)}
      noValidate
    >
      <div className="relative">
        {mentionQuery && mentionCandidates.length > 0 ? (
          <div
            role="listbox"
            aria-label={t("messageMentionCandidates")}
            className="absolute bottom-full left-2 z-30 mb-1 max-h-56 min-w-56 overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
          >
            {mentionCandidates.map((participant, index) => (
              <button
                key={participant.chatSubjectId}
                type="button"
                role="option"
                aria-selected={index === activeMentionIndex}
                className="flex w-full items-center rounded-sm px-2 py-1.5 text-left text-sm outline-none hover:bg-accent aria-selected:bg-accent"
                onPointerDown={(event) => event.preventDefault()}
                onClick={() => selectMention(participant)}
              >
                {participant.displayName}
              </button>
            ))}
          </div>
        ) : null}
        <button
          type="button"
          className="absolute inset-x-0 top-0 z-10 h-3 -translate-y-1/2 cursor-row-resize touch-none border-0 bg-transparent p-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t("composerResize")}
          onPointerDown={startInputResize}
          onPointerMove={resizeInput}
          onPointerUp={stopInputResize}
          onPointerCancel={stopInputResize}
          onKeyDown={resizeInputFromKeyboard}
        />
        <div className="overflow-hidden rounded-xl border border-input bg-muted/15 shadow-xs">
          {conversationType === ConversationType.ConversationTypeGroup &&
          replyTo ? (
            <div className="flex items-start justify-between gap-3 border-b px-3 py-2 text-xs">
              <div className="min-w-0">
                <p className="font-medium text-foreground">
                  {t("messageReplyingTo", {
                    name:
                      replyTo.sender?.displayName?.trim() ||
                      t("unknownSender"),
                  })}
                </p>
                <p className="truncate text-muted-foreground">
                  {replyTo.body}
                </p>
              </div>
              <button
                type="button"
                className="shrink-0 text-muted-foreground hover:text-foreground"
                onClick={() => onReplyToChange?.(null)}
              >
                {t("messageReplyCancel")}
              </button>
            </div>
          ) : null}
          <Textarea
            {...bodyField}
            ref={(input) => {
              bodyField.ref(input)
              inputRef.current = input
            }}
            id={inputID}
            disabled={isSubmitting}
            rows={3}
            required
            aria-label={t("replyLabel")}
            aria-invalid={form.formState.errors.body ? true : undefined}
            className="min-h-20 max-h-[200px] resize-none rounded-none border-0 bg-transparent py-2 shadow-none focus-visible:ring-0"
            onInput={(event) => {
              resizeComposerInput(
                event.currentTarget,
                manualInputHeightRef.current,
              )
            }}
            onChange={(event) => {
              bodyField.onChange(event)
              reconcileMentionSubjects(event.currentTarget.value)
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
          <div className="flex items-center justify-between gap-3 px-2.5 pb-2.5">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              disabled
              aria-label={t("attachmentAdd")}
              title={t("attachmentAdd")}
            >
              <PaperclipIcon />
            </Button>
            <Button type="submit" size="sm" disabled={isSubmitting}>
              {isSubmitting && showSubmitting ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : null}
              {isSubmitting && showSubmitting
                ? t("messageSending")
                : t("messageSend")}
            </Button>
          </div>
        </div>
      </div>
    </form>
  )
}

/** 展示保持工作区高度稳定的不可用回复区。 */
export function ConversationComposerUnavailable({
  conversationID,
  reason,
}: {
  conversationID: string
  reason: string
}) {
  const { t } = useTranslation("inbox")
  const inputID = `conversation-reply-${conversationID}`
  const reasonID = `${inputID}-reason`

  return (
    <div
      data-slot="conversation-composer"
      data-conversation-id={conversationID}
      aria-disabled="true"
      className="shrink-0 bg-background p-3"
    >
      <div className="overflow-hidden rounded-xl border border-input bg-muted/25 shadow-xs">
        <Textarea
          id={inputID}
          disabled
          rows={3}
          aria-label={t("replyLabel")}
          aria-describedby={reasonID}
          className="min-h-20 resize-none rounded-none border-0 bg-transparent py-2 shadow-none disabled:opacity-100"
        />
        <div className="flex min-h-8 items-center justify-between gap-3 px-3 pb-2.5">
          <p id={reasonID} className="text-xs text-muted-foreground">
            {reason}
          </p>
          <Button type="button" size="sm" disabled>
            {t("messageSend")}
          </Button>
        </div>
      </div>
    </div>
  )
}
