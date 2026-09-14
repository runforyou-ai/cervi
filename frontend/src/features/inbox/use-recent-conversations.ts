/** 当前设备记录的最近打开会话。 */
import { useCallback, useState } from "react"

const recentConversationLimit = 6

/** 读取并更新当前身份在本机最近打开的会话编号，最新打开的排在最前。 */
export function useRecentConversations(identityId: string) {
  const storageKey = `cervi.inbox.recent.${identityId}`
  const [ids, setIds] = useState<string[]>(() => {
    try {
      const stored: unknown = JSON.parse(localStorage.getItem(storageKey) ?? "[]")
      return Array.isArray(stored)
        ? stored.filter((id): id is string => typeof id === "string").slice(0, recentConversationLimit)
        : []
    } catch {
      return []
    }
  })

  const record = useCallback(
    (conversationId: string) => {
      setIds((current) => {
        const next = [conversationId, ...current.filter((id) => id !== conversationId)].slice(0, recentConversationLimit)
        try {
          localStorage.setItem(storageKey, JSON.stringify(next))
        } catch {
          // 本机存储不可用时只在当前页面保留记录。
        }
        return next
      })
    },
    [storageKey],
  )

  return { ids, record }
}
