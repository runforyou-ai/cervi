/** 对客回复与内部备注两种输入模式各自保留正文和提醒成员，切换模式时保存并载入对应草稿。 */
import {
  useCallback,
  useEffect,
  useRef,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react"
import type { UseFormReturn } from "react-hook-form"

import type { MessageVisibility } from "@/api"
import type { ConversationComposerValues } from "@/features/inbox/conversation-composer-schema"
import type { MentionTarget } from "@/features/inbox/outgoing-message-store"

import { resizeComposerInput } from "./composer-input"

/** 按可见范围保存和载入输入框草稿，并提供切换模式的入口。 */
export function useVisibilityDrafts({
  form,
  visibility,
  onVisibilityChange,
  inputRef,
  mentionsRef,
  setMentions,
  closeMentionQuery,
}: {
  form: UseFormReturn<ConversationComposerValues>
  visibility: MessageVisibility
  onVisibilityChange?: (visibility: MessageVisibility) => void
  inputRef: RefObject<HTMLTextAreaElement | null>
  mentionsRef: RefObject<MentionTarget[]>
  setMentions: Dispatch<SetStateAction<MentionTarget[]>>
  closeMentionQuery: () => void
}) {
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
    closeMentionQuery()
    delete draftsRef.current[visibility]
    delete draftMentionsRef.current[visibility]
    const focus = focusAfterSwitchRef.current
    focusAfterSwitchRef.current = false
    window.requestAnimationFrame(() => {
      resizeComposerInput(inputRef.current)
      if (focus) form.setFocus("body")
    })
  }, [form, visibility])

  /** 切换输入模式并把焦点留在输入框。 */
  function switchVisibility(next: MessageVisibility) {
    if (next === visibility) return
    focusAfterSwitchRef.current = true
    onVisibilityChange?.(next)
  }

  /** 把正文和提醒成员存入指定可见范围的草稿，切换到该模式时载入。 */
  const stashDraft = useCallback(
    (target: MessageVisibility, body: string, mentions: MentionTarget[]) => {
      draftsRef.current[target] = body
      draftMentionsRef.current[target] = mentions
    },
    [],
  )

  return { draftsRef, focusAfterSwitchRef, switchVisibility, stashDraft }
}
