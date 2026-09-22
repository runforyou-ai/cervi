/** 时间线与回复区之间的发送状态、引用目标和失败重试接线。 */
import { useRef, useState } from "react"

import {
  MessageVisibility,
  type ConversationMessageReference,
} from "@/api"
import { useOutgoingMessages } from "@/features/inbox/outgoing-message-context"
import type { OutgoingConversationDraft } from "@/features/inbox/outgoing-message-store"

/**
 * 按会话绑定发送项（尚无会话编号时按 draftKey 分组），对客回复与内部备注各自保留引用目标；
 * timeline 与 composer 分别展开到 ConversationTimeline 与 ConversationComposer。
 */
export function useThreadComposerBridge(conversationKey: string, draftKey = "") {
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const outgoing = useOutgoingMessages(conversationKey, draftKey)
  const [visibility, setVisibility] = useState<MessageVisibility>(
    MessageVisibility.MessageVisibilityCustomerVisible,
  )
  const [replyTargets, setReplyTargets] = useState<
    Partial<Record<MessageVisibility, ConversationMessageReference | null>>
  >({})
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const replyTo = replyTargets[visibility] ?? null

  /** 保存指定输入模式的引用目标并切到该模式。 */
  function selectReplyTarget(
    message: ConversationMessageReference | null,
    target: MessageVisibility = visibility,
  ) {
    setVisibility(target)
    setReplyTargets((current) => ({ ...current, [target]: message }))
  }

  return {
    outgoing,
    visibility,
    setVisibility,
    selectReplyTarget,
    timeline: {
      prepareSendRef,
      outgoingMessages: outgoing.messages,
      /** 失败消息回到发送时的输入模式和引用目标，交给回复区重新填入。 */
      onRetryFailedMessage: (draft: OutgoingConversationDraft) => {
        selectReplyTarget(draft.replyTo, draft.visibility)
        setRetryDraft(draft)
      },
    },
    composer: {
      onBeforeSend: () => prepareSendRef.current?.() ?? Promise.resolve(true),
      retryDraft,
      replyTo,
      visibility,
      onRetryDraftHandled: () => setRetryDraft(null),
      onReplyToChange: (message: ConversationMessageReference | null) =>
        selectReplyTarget(message),
      onSending: outgoing.start,
      onSent: outgoing.succeed,
      onFailed: outgoing.fail,
    },
  }
}
