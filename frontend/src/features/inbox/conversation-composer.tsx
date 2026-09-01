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

import {
  ConversationType,
  sendCustomerTextMessage,
  sendDirectTextMessage,
  sendGroupTextMessage,
  type ConversationMessage,
} from "@/api"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import {
  createConversationComposerSchema,
  type ConversationComposerValues,
} from "@/features/inbox/conversation-composer-schema"
import {
  conversationSendingIndicatorDelay,
  type OutgoingConversationDraft,
} from "@/features/inbox/use-outgoing-conversation-messages"
import { recoverSession } from "@/lib/session-navigation"

const conversationComposerMaxHeight = 200
const conversationComposerMinHeight = 80
const conversationComposerKeyboardResizeStep = 16

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
  onRetryDraftHandled,
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
  onRetryDraftHandled?: () => void
  onSending: (message: OutgoingConversationDraft) => void
  onSent: (clientMessageID: string, message: ConversationMessage) => void
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
    resizeComposerInput(inputRef.current, manualInputHeightRef.current)
    form.setFocus("body")
  }, [
    form,
    isSubmitting,
    onRetryDraftHandled,
    retryDraft,
    retryFailedMessage,
  ])

  useEffect(() => {
    if (isSubmitting || !refocusPendingRef.current) return
    refocusPendingRef.current = false
    form.setFocus("body")
  }, [form, isSubmitting])

  /** 按会话类型发送当前成员文本消息。 */
  async function send(values: ConversationComposerValues) {
    const body = values.body.trim()
    const retry =
      retryFailedMessage && retryRef.current?.body === body
        ? retryRef.current
        : null
    const draft = {
      clientMessageID: retry?.clientMessageID ?? window.crypto.randomUUID(),
      body,
      originatedAt: retry?.originatedAt ?? new Date().toISOString(),
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
      let message: ConversationMessage
      switch (conversationType) {
        case ConversationType.ConversationTypeDirect:
          message = await sendDirectTextMessage(conversationID, messageInput)
          break
        case ConversationType.ConversationTypeGroup:
          message = await sendGroupTextMessage(conversationID, messageInput)
          break
        case ConversationType.ConversationTypeCustomer:
          message = await sendCustomerTextMessage(conversationID, messageInput)
          break
        default:
          throw new Error("不支持的会话类型")
      }
      onSucceeded()
      if (!aliveRef.current) return
      refocusPendingRef.current = refocusAfterSubmit
      onSent(clientMessageID, message)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (!aliveRef.current) return
      console.warn("发送成员会话消息失败", {
        conversationId: conversationID,
        error,
      })
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
    if (
      !submitOnEnter ||
      event.key !== "Enter" ||
      event.shiftKey ||
      event.keyCode === 229 ||
      event.nativeEvent.isComposing
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
