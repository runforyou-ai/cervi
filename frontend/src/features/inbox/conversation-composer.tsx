/** 提交成员可回复会话的文本消息。 */
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { ArrowUpIcon, LoaderCircleIcon, MicIcon, PaperclipIcon, SmileIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { messagePreview } from "@/lib/message-preview"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ChatSubjectKind,
  ConversationType,
  MessageVisibility,
  OrganizationIdentityType,
  isApiError,
  sendCustomerTextMessage,
  sendAgentTextMessage,
  sendCustomerCopilotTextMessage,
  sendDirectTextMessage,
  sendGroupTextMessage,
  type ConversationMessageData,
  type ConversationMessageReference,
  type DirectTextMessageInput,
  type GroupParticipant,
  type InboxConversation,
  type MemberOption,
} from "@/api"
import { Button } from "@/components/ui/button"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { Textarea } from "@/components/ui/textarea"
import {
  createConversationComposerSchema,
  type ConversationComposerValues,
} from "@/features/inbox/conversation-composer-schema"
import {
  mentionTokenPattern,
  reconcileMentionAllToken,
  type MentionAllToken,
} from "@/lib/mention-token"
import {
  conversationSendingIndicatorDelay,
  type MentionTarget,
  type OutgoingConversationDraft,
} from "@/features/inbox/outgoing-message-store"
import { ConversationAttachmentUpload } from "./conversation-attachment-upload"
import { CustomerReplyAssistant } from "@/features/inbox/customer-reply-assistant"
import { composerToolClass } from "@/features/inbox/composer-tool"
import { useConversationTypingReport } from "@/features/inbox/use-conversation-typing"
import { resolveAppPlatform } from "@/platform/app-platform"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"
import composerEmojis from "../../../../internal/publicweb/composer-emojis.json"

const conversationComposerMaxHeight = 200
const conversationComposerMinHeight = 80
const conversationComposerKeyboardResizeStep = 16

/** 读取和替换回复输入框草稿的入口。 */
export type ComposerDraftBridge = {
  read: () => string
  replace: (body: string) => void
}

type MentionCandidate =
  | { kind: "all"; displayName: string }
  | { kind: "member"; displayName: string; target: MentionTarget }

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
  customerAttachmentSupported = false,
  customerTypingSupported = false,
  customerAttachmentByteLimit = 0,
  customerAttachmentCaptionLimit = 4000,
  onAttachmentConversationCreated,
  draftBridgeRef,
}: {
  attachmentTargetIdentityID?: string
  attachmentAgentDraft?: { conversationID: string; agentIdentityID: string; customerConversationID?: string }
  customerAttachmentSupported?: boolean
  customerTypingSupported?: boolean
  customerAttachmentByteLimit?: number
  customerAttachmentCaptionLimit?: number
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
  const manualInputHeightRef = useRef<number | null>(null)
  const resizeStartRef = useRef<{
    pointerY: number
    inputHeight: number
  } | null>(null)
  const retryRef = useRef<OutgoingConversationDraft | null>(null)
  const refocusPendingRef = useRef(false)
  const replyToRef = useRef(replyTo)
  replyToRef.current = replyTo
  const visibilityRef = useRef(visibility)
  visibilityRef.current = visibility
  const [mentions, setMentions] = useState<MentionTarget[]>([])
  const mentionsRef = useRef(mentions)
  mentionsRef.current = mentions
  const [mentionAllToken, setMentionAllToken] =
    useState<MentionAllToken | null>(null)
  const mentionAll = mentionAllToken !== null
  const [mentionQuery, setMentionQuery] = useState<{
    start: number
    value: string
  } | null>(null)
  const [activeMentionIndex, setActiveMentionIndex] = useState(0)
  const [emojiOpen, setEmojiOpen] = useState(false)
  const emojiCaretRef = useRef<number | null>(null)
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
  // 对客草稿与内部备注草稿各自保留正文和提醒成员，切换页签时互不覆盖。
  const draftsRef = useRef<Partial<Record<MessageVisibility, string>>>({})
  const draftMentionsRef = useRef<Partial<Record<MessageVisibility, MentionTarget[]>>>({})
  const appliedVisibilityRef = useRef(visibility)
  const focusAfterSwitchRef = useRef(false)

  // 页签切换和引用、填入回复引起的模式变化共用同一套草稿保存与载入。
  useEffect(() => {
    const previous = appliedVisibilityRef.current
    if (previous === visibility) return
    appliedVisibilityRef.current = visibility
    draftsRef.current[previous] = form.getValues("body")
    draftMentionsRef.current[previous] = mentionsRef.current
    form.setValue("body", draftsRef.current[visibility] ?? "")
    setMentions(draftMentionsRef.current[visibility] ?? [])
    setMentionQuery(null)
    delete draftsRef.current[visibility]
    delete draftMentionsRef.current[visibility]
    const focus = focusAfterSwitchRef.current
    focusAfterSwitchRef.current = false
    window.requestAnimationFrame(() => {
      resizeComposerInput(inputRef.current, manualInputHeightRef.current)
      if (focus) form.setFocus("body")
    })
  }, [form, visibility])

  /** 切换输入模式并把焦点留在输入框。 */
  function switchVisibility(next: MessageVisibility) {
    if (next === visibility) return
    focusAfterSwitchRef.current = true
    onVisibilityChange?.(next)
  }

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
    // 失败消息回到发送时的可见范围，当前页签属于另一种可见范围时先存入对应草稿。
    if (retryDraft.visibility !== visibility) {
      draftsRef.current[retryDraft.visibility] = retryDraft.body
      draftMentionsRef.current[retryDraft.visibility] = retryDraft.mentions
      return
    }
    form.setValue("body", retryDraft.body, { shouldDirty: true })
    setMentions(retryDraft.mentions)
    setMentionAllToken(retryDraft.mentionAllToken)
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
    visibility,
  ])

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
  // 群聊提醒当前成员，客户会话的内部备注提醒企业真人成员。
  const mentionTargets = useMemo<MentionTarget[]>(() => {
    if (groupConversation) {
      return (groupParticipants ?? []).map((participant) => ({
        identityID: participant.identityId,
        chatSubjectID: participant.chatSubjectId,
        displayName: participant.displayName,
      }))
    }
    if (!customerConversation || !internalNote) return []
    return (noteMentionMembers ?? [])
      .filter((member) => member.type === OrganizationIdentityType.OrganizationIdentityTypeUser)
      .map((member) => ({ identityID: member.id, chatSubjectID: null, displayName: member.displayName }))
  }, [customerConversation, groupConversation, groupParticipants, internalNote, noteMentionMembers])

  const mentionCandidates = useMemo<MentionCandidate[]>(() => {
    if (!mentionQuery) return []
    const query = mentionQuery.value.toLocaleLowerCase()
    const candidates: MentionCandidate[] = []
    if (groupConversation && !mentionAll && t("messageMentionAll").toLocaleLowerCase().includes(query)) {
      candidates.push({ kind: "all", displayName: t("messageMentionAll") })
    }
    candidates.push(
      ...mentionTargets
        .filter(
          (target) =>
            target.identityID !== currentIdentityID &&
            !mentions.some((mention) => mention.identityID === target.identityID) &&
            target.displayName.toLocaleLowerCase().includes(query),
        )
        .map((target) => ({
          kind: "member" as const,
          displayName: target.displayName,
          target,
        })),
    )
    return candidates.slice(0, 8)
  }, [
    currentIdentityID,
    groupConversation,
    mentionTargets,
    mentionQuery,
    mentions,
    mentionAll,
    t,
  ])
  // 对客模式输入 @ 时提示切换到内部备注提醒同事。
  const noteMentionHint =
    customerConversation && !internalNote && Boolean(onVisibilityChange) && mentionQuery !== null

  /** 根据光标前文本更新 @ 候选查询。 */
  function updateMentionQuery(value: string, selectionStart: number | null) {
    if (
      (!groupConversation && !customerConversation) ||
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
  function reconcileMentions(value: string) {
    setMentions((current) => {
      const remainingByName = new Map<string, number>()
      return current.filter((mention) => {
        const remaining =
          remainingByName.get(mention.displayName) ??
          countMentionTokens(value, mention.displayName)
        remainingByName.set(mention.displayName, Math.max(remaining - 1, 0))
        return remaining > 0
      })
    })
  }

  /** 在正文光标处插入选中的成员或所有人标记。 */
  function selectMention(candidate: MentionCandidate) {
    const query = mentionQuery
    const input = inputRef.current
    if (!query || !input) return
    const body = form.getValues("body")
    const caret = input.selectionStart ?? body.length
    const token = `@${candidate.displayName} `
    const nextBody = `${body.slice(0, query.start)}${token}${body.slice(caret)}`
    const nextCaret = query.start + token.length
    form.setValue("body", nextBody, { shouldDirty: true })
    typingReport.input(nextBody)
    if (candidate.kind === "all") {
      setMentionAllToken({ start: query.start, text: token.trimEnd() })
    } else {
      setMentionAllToken((current) =>
        reconcileMentionAllToken(current, body, nextBody, nextCaret),
      )
      const { target } = candidate
      setMentions((current) =>
        current.some((mention) => mention.identityID === target.identityID)
          ? current
          : [...current, target],
      )
    }
    setMentionQuery(null)
    window.requestAnimationFrame(() => {
      input.focus()
      input.setSelectionRange(nextCaret, nextCaret)
      resizeComposerInput(input, manualInputHeightRef.current)
    })
  }

  /** 用选中的表情替换正文当前选区，关闭面板后光标落在表情之后。 */
  function insertEmoji(emoji: string) {
    const input = inputRef.current
    if (!input) return
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
    resizeComposerInput(input, manualInputHeightRef.current)
    emojiCaretRef.current = nextCaret
    setEmojiOpen(false)
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

  useEffect(() => {
    if (disabledReason || isSubmitting) return
    // 焦点不在输入控件上时，按下可打印字符直接转交输入框，该字符落入输入框。
    function focusFromTyping(event: globalThis.KeyboardEvent) {
      if (event.defaultPrevented || event.isComposing) return
      if (event.ctrlKey || event.metaKey || event.altKey) return
      // 空格是按钮和复选框的激活键，留给当前焦点元素。
      if (event.key.length !== 1 || event.key === " ") return
      const input = inputRef.current
      if (!input || input.closest("[aria-hidden='true']")) return
      const active = document.activeElement
      if (
        active instanceof HTMLElement &&
        (active.isContentEditable ||
          active.tagName === "INPUT" ||
          active.tagName === "TEXTAREA" ||
          active.tagName === "SELECT" ||
          // 对话框和各类浮层内的按键归浮层处理。
          active.closest("[data-radix-popper-content-wrapper],[role='dialog']"))
      )
        return
      input.focus()
      input.setSelectionRange(input.value.length, input.value.length)
    }
    document.addEventListener("keydown", focusFromTyping)
    return () => document.removeEventListener("keydown", focusFromTyping)
  }, [disabledReason, isSubmitting])

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
    resizeComposerInput(inputRef.current, manualInputHeightRef.current)
    try {
      const messageInput = { clientMessageId: clientMessageID, body }
      let message: ConversationMessageData
      switch (conversationType) {
        case ConversationType.ConversationTypeAgent:
        case ConversationType.ConversationTypeCopilot:
        case ConversationType.ConversationTypeDirect: {
          const directInput = {
            ...messageInput,
            replyToMessageId: replyTo?.id ?? "",
          }
          // 草稿首发走调用方入口，已有会话按类型发送。
          if (sendIndividualMessage) message = await sendIndividualMessage(directInput)
          else if (conversationType === ConversationType.ConversationTypeAgent)
            message = await sendAgentTextMessage(conversationID, directInput)
          else if (conversationType === ConversationType.ConversationTypeCopilot)
            message = await sendCustomerCopilotTextMessage(conversationID, directInput)
          else message = await sendDirectTextMessage(conversationID, directInput)
          break
        }
        case ConversationType.ConversationTypeGroup:
          message = await sendGroupTextMessage(conversationID, {
            ...messageInput,
            replyToMessageId: replyTo?.id ?? "",
            mentionSubjectIds: orderedMentions.flatMap((mention) =>
              mention.chatSubjectID ? [mention.chatSubjectID] : [],
            ),
            mentionAll,
          })
          break
        case ConversationType.ConversationTypeCustomer:
          message = await sendCustomerTextMessage(conversationID, {
            ...messageInput,
            replyToMessageId: replyTo?.id ?? "",
            visibility,
            mentionIdentityIds: internalNote
              ? orderedMentions.map((mention) => mention.identityID)
              : [],
          })
          break
        default:
          throw new Error("不支持的会话类型")
      }
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
          draftsRef.current[draft.visibility] = body
          draftMentionsRef.current[draft.visibility] = draft.mentions
        } else {
          form.setValue("body", body, { shouldDirty: true })
          setMentions(draft.mentions)
          setMentionAllToken(draft.mentionAllToken)
          resizeComposerInput(inputRef.current, manualInputHeightRef.current)
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
      resizeComposerInput(inputRef.current, manualInputHeightRef.current)
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

  // 输入内容后附件入口换成发送按钮，并保持到本次发送结束。
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
        // 行高贴近字体自然行高，避免换行前后光标高度跳变；与上下内边距之和等于单行高度，正文与工具图标同一水平线。
        "max-h-[200px] min-h-9 min-w-0 flex-1 resize-none rounded-none border-0 bg-transparent px-0.5 py-2 leading-5 shadow-none caret-primary focus-visible:ring-0 dark:bg-transparent",
        // 正文与 20px 工具图标配比；窄屏保持 16px，避免移动端聚焦时缩放。
        "md:text-[15px]",
        disabledReason && "pl-3",
      )}
      onInput={(event) => {
        resizeComposerInput(
          event.currentTarget,
          manualInputHeightRef.current,
        )
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
  const attachmentTool =
    (conversationType === ConversationType.ConversationTypeDirect ||
      conversationType === ConversationType.ConversationTypeAgent ||
      conversationType === ConversationType.ConversationTypeCopilot ||
      conversationType === ConversationType.ConversationTypeGroup ||
      (customerAttachmentSupported && !internalNote)) ? (
      <ConversationAttachmentUpload
        conversationID={conversationID || (attachmentAgentDraft?.conversationID ?? "")}
        targetIdentityID={attachmentTargetIdentityID}
        agentIdentityID={attachmentAgentDraft?.agentIdentityID}
        customerConversationID={attachmentAgentDraft?.customerConversationID}
        customer={conversationType === ConversationType.ConversationTypeCustomer}
        byteLimit={customerAttachmentByteLimit}
        captionLimit={customerAttachmentCaptionLimit}
        replyTo={replyTo ?? null}
        onSent={() => onReplyToChange?.(null)}
        disabled={isSubmitting}
        onBeforeSend={onBeforeSend}
        onCreated={(conversation, conversationID) => onAttachmentConversationCreated?.(conversation, conversationID)}
      />
    ) : (
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        className={composerToolClass}
        disabled
        aria-label={t("attachmentAdd")}
      >
        <PaperclipIcon />
      </Button>
    )
  const emojiTool = (
    <Popover open={emojiOpen} onOpenChange={setEmojiOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className={composerToolClass}
          disabled={isSubmitting}
          aria-label={t("emojiPick")}
        >
          <SmileIcon className="size-[18px]" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        collisionPadding={8}
        aria-label={t("emojiPick")}
        className={cn(
          "grid max-h-64 gap-0.5 overflow-y-auto p-1.5",
          // 移动端面板不超过可用宽度，列数按 44px 触控按钮自动填充。
          mobile
            ? "w-[min(22rem,var(--radix-popover-content-available-width))] grid-cols-[repeat(auto-fill,2.75rem)]"
            : "w-auto grid-cols-8",
        )}
        onCloseAutoFocus={(event) => {
          // 选中表情后焦点回到输入框并定位到插入内容之后。
          const caret = emojiCaretRef.current
          const input = inputRef.current
          if (caret === null || !input) return
          emojiCaretRef.current = null
          event.preventDefault()
          input.focus()
          input.setSelectionRange(caret, caret)
        }}
      >
        {composerEmojis.map((emoji) => (
          <button
            key={emoji}
            type="button"
            className={cn(
              "flex items-center justify-center rounded-md text-xl leading-none outline-none hover:bg-accent focus-visible:bg-accent",
              mobile ? "size-11" : "size-8",
            )}
            onClick={() => insertEmoji(emoji)}
          >
            {emoji}
          </button>
        ))}
      </PopoverContent>
    </Popover>
  )

  return (
    <form
      data-slot="conversation-composer"
      data-conversation-id={conversationID}
      className="shrink-0 bg-background"
      onSubmit={form.handleSubmit(send)}
      noValidate
    >
      {onVisibilityChange ? (
        <div role="tablist" aria-label={t("composerMode")} className="flex items-center gap-1 px-2 pt-2 pb-1.5">
          <button
            type="button"
            role="tab"
            aria-selected={!internalNote}
            className={cn(
              "rounded-md px-2 py-1 text-xs font-medium",
              mobile && "min-h-11 px-3 text-sm",
              internalNote
                ? "text-muted-foreground hover:text-foreground"
                : "bg-muted text-foreground",
            )}
            onClick={() =>
              switchVisibility(MessageVisibility.MessageVisibilityCustomerVisible)
            }
          >
            {t("composerModeCustomer")}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={internalNote}
            className={cn(
              "rounded-md px-2 py-1 text-xs font-medium",
              mobile && "min-h-11 px-3 text-sm",
              internalNote
                ? "bg-amber-100 text-amber-900 dark:bg-amber-950 dark:text-amber-200"
                : "text-muted-foreground hover:text-foreground",
            )}
            onClick={() =>
              switchVisibility(MessageVisibility.MessageVisibilityInternalOnly)
            }
          >
            {t("composerModeNote")}
          </button>
        </div>
      ) : null}
      <div className="relative">
        {!disabledReason && mentionQuery && mentionCandidates.length > 0 ? (
          <div
            role="listbox"
            aria-label={t(groupConversation ? "messageMentionCandidates" : "noteMentionCandidates")}
            className="absolute bottom-full left-2 z-30 mb-1 max-h-56 min-w-56 overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
          >
            {mentionCandidates.map((candidate, index) => (
              <button
                key={
                  candidate.kind === "all"
                    ? "all"
                    : candidate.target.identityID
                }
                type="button"
                role="option"
                aria-selected={index === activeMentionIndex}
                className={cn(
                  "flex w-full items-center rounded-sm px-2 py-1.5 text-left text-sm outline-none hover:bg-accent aria-selected:bg-accent",
                  mobile && "min-h-11",
                )}
                onPointerDown={(event) => event.preventDefault()}
                onClick={() => selectMention(candidate)}
              >
                {candidate.displayName}
              </button>
            ))}
          </div>
        ) : null}
        {!disabledReason && noteMentionHint ? (
          <div
            role="status"
            className="absolute bottom-full left-2 z-30 mb-1 flex items-center gap-3 rounded-md border bg-popover py-1.5 pr-1.5 pl-3 text-sm text-popover-foreground shadow-md"
          >
            {t("noteMentionHint")}
            <Button
              type="button"
              size="sm"
              variant="outline"
              onPointerDown={(event) => event.preventDefault()}
              onClick={() =>
                switchVisibility(MessageVisibility.MessageVisibilityInternalOnly)
              }
            >
              {t("noteMentionSwitch")}
            </Button>
          </div>
        ) : null}
        {mobile ? null : (
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
        )}
        <div
          className={cn(
            "border-t",
            internalNote
              ? "border-amber-500/70 bg-amber-50/60 dark:bg-amber-950/30"
              : "border-input bg-background",
          )}
        >
          {replyTo ? (
            <div className="flex items-start justify-between gap-3 border-b px-3 py-2 text-xs">
              <div className="min-w-0">
                <p className="font-medium text-foreground">
                  {replyTo.deleted
                    ? t("messageOriginalDeleted")
                    : t("messageReplyingTo", {
                        name:
                          replyTo.sender?.displayName?.trim() ||
                          t(replyTo.sender?.kind === ChatSubjectKind.ChatSubjectKindContact ? "anonymousVisitor" : "unknownSender"),
                      })}
                </p>
                <p className="truncate text-muted-foreground">
                  {messagePreview(replyTo.body, replyTo.sender?.identityType)}
                </p>
              </div>
              <button
                type="button"
                className={cn(
                  "shrink-0 text-muted-foreground hover:text-foreground",
                  mobile && "-my-2 min-h-11 px-2",
                )}
                disabled={Boolean(disabledReason)}
                onClick={() => onReplyToChange?.(null)}
              >
                {t("messageReplyCancel")}
              </button>
            </div>
          ) : null}
          {disabledReason ? (
            <p
              id={`${inputID}-reason`}
              className="truncate border-b px-3 py-1.5 text-xs text-muted-foreground"
            >
              {disabledReason}
            </p>
          ) : null}
          <div className="flex items-end gap-1 px-2 py-1.5">
            {disabledReason ? null : attachmentTool}
            {bodyInput}
            {disabledReason ? null : emojiTool}
            {replyAssistant}
            {showSend ? (
              <Button
                type="submit"
                size="icon"
                className="relative size-9 rounded-full after:absolute after:-inset-1 after:content-[''] [&_svg:not([class*='size-'])]:size-5"
                disabled={isSubmitting || Boolean(disabledReason) || isBodyEmpty || replyTo?.deleted}
                aria-label={t(internalNote ? "internalNoteSave" : "messageSend")}
              >
                {showSubmitting ? <LoaderCircleIcon className="animate-spin" /> : <ArrowUpIcon />}
              </Button>
            ) : (
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
            )}
          </div>
        </div>
      </div>
    </form>
  )
}
