/** 提交 Customer 或 Direct 会话的成员文本消息。 */
import { useEffect, useMemo, useRef } from "react"
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
  type ConversationMessage,
} from "@/api"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import {
  createConversationComposerSchema,
  type ConversationComposerValues,
} from "@/features/inbox/conversation-composer-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

type PendingMessage = {
  body: string
  clientMessageID: string
}

/** 展示并提交成员会话文本编辑区。 */
export function ConversationComposer({
  conversationID,
  conversationType,
  onSent,
}: {
  conversationID: string
  conversationType: ConversationType
  onSent: (message: ConversationMessage) => void
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const aliveRef = useRef(true)
  const pendingRef = useRef<PendingMessage | null>(null)
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

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
    }
  }, [])

  /** 按会话类型发送当前成员文本消息。 */
  async function send(values: ConversationComposerValues) {
    const body = values.body.trim()
    const pending =
      pendingRef.current?.body === body
        ? pendingRef.current
        : { body, clientMessageID: window.crypto.randomUUID() }
    pendingRef.current = pending
    try {
      const message =
        conversationType === ConversationType.ConversationTypeDirect
          ? await sendDirectTextMessage(conversationID, {
              clientMessageId: pending.clientMessageID,
              body,
            })
          : await sendCustomerTextMessage(conversationID, {
              clientMessageId: pending.clientMessageID,
              body,
            })
      void invalidate(resourceKeys.inbox())
      if (!aliveRef.current) return
      pendingRef.current = null
      form.reset()
      onSent(message)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (!aliveRef.current) return
      console.warn("发送成员会话消息失败", {
        conversationId: conversationID,
        error,
      })
      if (isApiError(error)) {
        if (error.reason === "idempotency_mismatch") {
          pendingRef.current = null
        }
        toast.error(apiErrorMessage(error, ["body", "clientMessageId"]))
        return
      }
      toast.error(t("messageSendError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form
      data-slot="conversation-composer"
      data-conversation-id={conversationID}
      className="shrink-0 border-t border-border/60 bg-background p-3"
      onSubmit={form.handleSubmit(send)}
      noValidate
    >
      <div className="overflow-hidden rounded-xl border border-input bg-muted/15 shadow-xs">
        <label
          htmlFor={inputID}
          className="block px-3 pt-2.5 text-xs font-medium text-muted-foreground"
        >
          {t("replyLabel")}
        </label>
        <Textarea
          {...form.register("body")}
          id={inputID}
          disabled={isSubmitting}
          rows={3}
          required
          aria-invalid={form.formState.errors.body ? true : undefined}
          className="min-h-20 resize-none rounded-none border-0 bg-transparent py-2 shadow-none focus-visible:ring-0"
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
            {isSubmitting ? (
              <LoaderCircleIcon className="animate-spin" />
            ) : null}
            {isSubmitting ? t("messageSending") : t("messageSend")}
          </Button>
        </div>
      </div>
    </form>
  )
}
