/** 成员端会话输入状态：输入框上报本人输入，会话头展示其他成员或访客正在输入。 */
import { useEffect, useMemo, useRef, useSyncExternalStore } from "react"
import { useTranslation } from "react-i18next"

import { reportConversationTyping, type GroupParticipant } from "@/api"
import { realtimeClient } from "@/api/realtime"
import { ConversationTypingReporter } from "@/features/inbox/conversation-typing-reporter"
import { ConversationTypingStore } from "@/features/inbox/conversation-typing-store"

const typingStore = new ConversationTypingStore()

realtimeClient.subscribe((event) => {
  if (event.type === "frame" && event.frame.type === "conversation_typing") {
    typingStore.apply(event.frame.conversationId, event.frame.senderSubjectId, event.frame.active)
  }
})

/** 新消息到达后清除其发送者的输入状态。 */
export function clearConversationTypingSender(conversationID: string, senderSubjectID: string) {
  typingStore.clear(conversationID, senderSubjectID)
}

/** 返回输入框上报本人输入状态的入口；切换会话、卸载或页面隐藏时上报停止输入。 */
export function useConversationTypingReport(conversationID: string, enabled: boolean) {
  const reporterRef = useRef<ConversationTypingReporter | null>(null)

  useEffect(() => {
    if (!enabled || !conversationID) return
    const reporter = new ConversationTypingReporter((active) => {
      // 输入状态由接收端到期清除兜底，上报失败不影响输入与发送。
      reportConversationTyping(conversationID, active).catch(() => undefined)
    })
    reporterRef.current = reporter
    // 页面隐藏时结束本次输入。
    const stopWhenHidden = () => {
      if (document.visibilityState === "hidden") reporter.stop()
    }
    document.addEventListener("visibilitychange", stopWhenHidden)
    return () => {
      document.removeEventListener("visibilitychange", stopWhenHidden)
      reporter.stop()
      reporterRef.current = null
    }
  }, [conversationID, enabled])

  return useMemo(
    () => ({
      input: (value: string) => reporterRef.current?.input(value),
      stop: () => reporterRef.current?.stop(),
    }),
    [],
  )
}

/** 返回会话头展示的正在输入文案；participants 为空表示单聊，群聊按参与者解析名称并合并多人。 */
export function useConversationTypingLabel(
  conversationID: string,
  participants: GroupParticipant[] | null,
) {
  const { t } = useTranslation("inbox")
  const senders = useSyncExternalStore(
    (listener) => typingStore.subscribe(conversationID, listener),
    () => typingStore.senders(conversationID),
  )
  if (senders.length === 0) return ""
  if (!participants) return t("typingDirect")
  const names = senders.flatMap((senderSubjectID) => {
    const name = participants.find((participant) => participant.chatSubjectId === senderSubjectID)?.displayName.trim()
    return name ? [name] : []
  })
  if (names.length === 0) return ""
  if (names.length === 1) return t("typingOne", { name: names[0] })
  if (names.length === 2) return t("typingTwo", { first: names[0], second: names[1] })
  return t("typingMany", { count: names.length })
}
