/** 会话内正在回复的 AI 员工：由消息时间线发布，会话头订阅后展示。 */
import { useSyncExternalStore } from "react"
import { useTranslation } from "react-i18next"

const activeAgents = new Map<string, readonly string[]>()
const listeners = new Map<string, Set<() => void>>()
const emptyAgents: readonly string[] = []

/** 发布指定会话当前正在回复或等待发言的 AI 员工名称。 */
export function publishConversationAgentActivity(conversationID: string, agentNames: string[]) {
  const current = activeAgents.get(conversationID) ?? emptyAgents
  if (current.length === agentNames.length && current.every((name, index) => name === agentNames[index])) {
    return
  }
  if (agentNames.length === 0) activeAgents.delete(conversationID)
  else activeAgents.set(conversationID, agentNames)
  for (const listener of [...(listeners.get(conversationID) ?? [])]) listener()
}

/** 订阅指定会话正在回复的 AI 员工名称。 */
function useConversationAgentNames(conversationID: string) {
  return useSyncExternalStore(
    (listener) => {
      let current = listeners.get(conversationID)
      if (!current) {
        current = new Set()
        listeners.set(conversationID, current)
      }
      current.add(listener)
      return () => {
        current.delete(listener)
        if (current.size === 0) listeners.delete(conversationID)
      }
    },
    () => activeAgents.get(conversationID) ?? emptyAgents,
  )
}

/** 返回会话头展示的 AI 员工回复文案，无 AI 回复时为空串。 */
export function useConversationAgentReplyLabel(conversationID: string) {
  const { t } = useTranslation("inbox")
  const names = useConversationAgentNames(conversationID)
  if (names.length === 0) return ""
  if (names.length === 1) return t("agentReplyingOne", { name: names[0] })
  return t("agentReplyingMany", { count: names.length })
}
