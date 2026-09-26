/** 会话消息提交、重试和异步完成后的草稿恢复。 */
import { useEffect, useRef, type RefObject } from "react"
import type { UseFormReturn } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { MessageVisibility, isApiError, type CustomerReplyTranslation } from "@/api"
import type { ConversationComposerProps } from "./conversation-composer-types"
import type { ConversationComposerValues } from "./conversation-composer-schema"
import type { MentionTarget, OutgoingConversationDraft } from "./outgoing-message-store"
import type { useComposerMentions } from "./use-composer-mentions"
import type { useVisibilityDrafts } from "./use-visibility-drafts"
import { resizeComposerInput } from "./composer-input"
import { sendComposerTextMessage } from "./composer-send"
import { useCustomerTranslation } from "./customer-translation"
import { mentionTokenPattern } from "@/lib/mention-token"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 提交当前草稿，并把失败结果恢复到发送时的可见范围。 */
export function useComposerSubmission({ props, form, inputRef, disabledReason, mentionsState, stashDraft, typingReport }: {
  props: ConversationComposerProps
  form: UseFormReturn<ConversationComposerValues>
  inputRef: RefObject<HTMLTextAreaElement | null>
  disabledReason: string | null
  mentionsState: ReturnType<typeof useComposerMentions>
  stashDraft: ReturnType<typeof useVisibilityDrafts>["stashDraft"]
  typingReport: { stop: () => void }
}) {
  const { conversationID, conversationType, service = false, refocusAfterSubmit = false,
    visibility = MessageVisibility.MessageVisibilityShared, retryDraft = null, replyTo = null,
    onRetryDraftHandled, onReplyToChange, onSending, onBeforeSend, onSent, onFailed, onSucceeded, sendIndividualMessage,
  } = props
  const { mentions, setMentions, mentionAllToken, setMentionAllToken, mentionAll, setMentionQuery } = mentionsState
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const customerTranslation = useCustomerTranslation()
  const aliveRef = useRef(false)
  const retryRef = useRef<OutgoingConversationDraft | null>(null)
  const refocusPendingRef = useRef(false)
  const replyToRef = useRef(replyTo)
  replyToRef.current = replyTo
  const visibilityRef = useRef(visibility)
  visibilityRef.current = visibility
  const { isSubmitting } = form.formState
  useEffect(() => {
    aliveRef.current = true
    resizeComposerInput(inputRef.current)
    return () => { aliveRef.current = false }
  }, [inputRef])

  useEffect(() => {
    if (!retryDraft) return
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
    stashDraft,
    visibility,
  ])

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

  /** 按会话类型发送当前成员文本消息；translation 为预览过的对客译文。 */
  async function send(values: ConversationComposerValues, translation: CustomerReplyTranslation | null = null) {
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
        service,
        conversationID,
        clientMessageID,
        body,
        replyToMessageID: replyTo?.id ?? "",
        visibility,
        mentions: orderedMentions,
        mentionAll,
        translate: Boolean(customerTranslation?.replyNeedsTranslation && customerTranslation.translateReply),
        translation,
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
      // 回复语言变化或无法确定时刷新翻译状态，客服据此重新预览或选择回复语言。
      if (isApiError(error) && (error.reason === "reply_language_changed" || error.reason === "customer_language_unknown"))
        customerTranslation?.refresh()
      if (!aliveRef.current) return
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "replyToMessageId",
              "mentionSubjectIds",
              "mentionIdentityIds",
              "body",
              "translation",
            ])
          : t("messageSendError"),
      )
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
      refocusPendingRef.current = refocusAfterSubmit
    }
  }

  return send
}
