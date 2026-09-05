/** 固定本轮提及序列，并在可见后按服务端顺序连续确认。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import {
  ConversationMentionReviewOutcome,
  getConversationNavigationState,
  isApiError,
  listPendingConversationMentions,
  markConversationMentionReviewed,
} from "@/api"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { memberChatPollingInterval } from "./use-member-chat-polling"

type MentionRound = {
  ids: string[]
  lastSequence: string
  index: number
  confirmed: Set<string>
  unavailable: Set<string>
}

/** 管理当前群聊的一轮提及导航和实时数量。 */
export function useConversationMentionNavigation({
  conversationID,
  enabled,
  pollingActive,
  locate,
  cancel,
  onUnavailable,
}: {
  conversationID: string
  enabled: boolean
  pollingActive: boolean
  locate: (id: string) => Promise<boolean>
  cancel: () => void
  onUnavailable: () => void
}) {
  const { t } = useTranslation("inbox")
  const invalidate = useResourceInvalidator()
  const [round, setRound] = useState<MentionRound | null>(null)
  const [busy, setBusy] = useState(false)
  const [needsResume, setNeedsResume] = useState(false)
  const operation = useRef(0)
  const currentRound = useRef(round)
  currentRound.current = round
  const state = useResource(
    resourceKeys.conversationNavigation(conversationID),
    (signal) => getConversationNavigationState(conversationID, signal),
    {
      enabled: enabled && pollingActive,
      refetchInterval:
        enabled && pollingActive ? memberChatPollingInterval : false,
      refetchOnWindowFocus: false,
    },
  )
  const { read } = state

  /** 结束本轮但保留时间线位置。 */
  const close = useCallback(() => {
    operation.current += 1
    cancel()
    currentRound.current = null
    setRound(null)
    setBusy(false)
    setNeedsResume(false)
  }, [cancel])

  useEffect(() => {
    if (
      round &&
      state.data &&
      BigInt(state.data.reviewedThroughSequence) > BigInt(round.lastSequence)
    )
      close()
  }, [state.data, round, close])
  useEffect(() => {
    if (
      isApiError(state.error) &&
      state.error.reason === "conversation_unavailable"
    ) {
      close()
      onUnavailable()
    }
  }, [state.error, close, onUnavailable])
  useEffect(
    () => () => {
      operation.current += 1
    },
    [conversationID],
  )

  /** 引用占用视口时暂停尚未确认的当前提及。 */
  const pause = useCallback(() => {
    operation.current += 1
    cancel()
    setBusy(false)
    const active = currentRound.current
    setNeedsResume(
      Boolean(active && !active.confirmed.has(active.ids[active.index])),
    )
  }, [cancel])

  /** 跳过失效目标，只有可见且确认成功后才允许下一条。 */
  async function visit(
    index: number,
    active = currentRound.current,
    direction = 1,
  ) {
    if (!active) return
    const revision = ++operation.current
    setBusy(true)
    setNeedsResume(false)
    let next = active
    try {
      for (
        let position = index;
        position >= 0 && position < next.ids.length;
        position += direction
      ) {
        const id = next.ids[position]
        if (next.unavailable.has(id)) continue
        next = { ...next, index: position }
        currentRound.current = next
        setRound(next)
        try {
          if (!(await locate(id)) || revision !== operation.current) return
          if (!next.confirmed.has(id)) {
            const result = await markConversationMentionReviewed(
              conversationID,
              id,
            )
            if (revision !== operation.current) return
            if (
              result.outcome ===
              ConversationMentionReviewOutcome.ConversationMentionUnavailable
            ) {
              next = {
                ...next,
                unavailable: new Set([...next.unavailable, id]),
              }
              continue
            }
            next = { ...next, confirmed: new Set([...next.confirmed, id]) }
          }
          currentRound.current = next
          setRound(next)
          void invalidate(resourceKeys.conversationNavigation(conversationID))
          return
        } catch (error) {
          if (revision !== operation.current) return
          if (isApiError(error) && error.reason === "message_unavailable") {
            next = { ...next, unavailable: new Set([...next.unavailable, id]) }
            toast.message(t("messageOriginalDeleted"))
            continue
          }
          throw error
        }
      }
      // 到达本轮末端时保留最后一个仍能回看的目标。
      const remaining = next.ids.reduce(
        (last, id, index) => (next.unavailable.has(id) ? last : index),
        -1,
      )
      if (remaining < 0) close()
      else {
        next = { ...next, index: remaining }
        currentRound.current = next
        setRound(next)
        if (
          !(await locate(next.ids[remaining])) ||
          revision !== operation.current
        )
          return
        setNeedsResume(!next.confirmed.has(next.ids[remaining]))
      }
      void invalidate(resourceKeys.conversationNavigation(conversationID))
    } catch (error) {
      if (revision !== operation.current) return
      if (isApiError(error) && error.reason === "conversation_unavailable") {
        close()
        onUnavailable()
        return
      }
      if (isApiError(error) && error.reason === "mention_progress_changed")
        close()
      else setNeedsResume(true)
      if (!isApiError(error) || !error.state)
        toast.error(
          error instanceof Error ? error.message : t("mentionNavigationError"),
        )
    } finally {
      if (revision === operation.current) setBusy(false)
    }
  }

  /** 用当次完整查询结果开始新的一轮。 */
  async function start() {
    if (busy) return
    const revision = ++operation.current
    setBusy(true)
    try {
      const pending = await read(
        resourceKeys.conversationMentions(conversationID),
        (signal) => listPendingConversationMentions(conversationID, signal),
      )
      if (revision !== operation.current) return
      if (!pending.messageIds.length || !pending.lastTargetSequence) {
        void invalidate(resourceKeys.conversationNavigation(conversationID))
        return
      }
      const next: MentionRound = {
        ids: pending.messageIds,
        lastSequence: pending.lastTargetSequence,
        index: 0,
        confirmed: new Set(),
        unavailable: new Set(),
      }
      await visit(0, next)
    } catch (error) {
      if (revision !== operation.current) return
      if (isApiError(error) && error.reason === "conversation_unavailable")
        onUnavailable()
      else if (!isApiError(error) || !error.state)
        toast.error(
          error instanceof Error ? error.message : t("mentionNavigationError"),
        )
    } finally {
      if (revision === operation.current) setBusy(false)
    }
  }

  return {
    round,
    busy,
    needsResume,
    pendingCount: state.data?.pendingMentionCount ?? 0,
    latestSequence: state.data?.latestSequence ?? "0",
    start,
    visit,
    close,
    pause,
  }
}
