/** 按会话内消息序号维护顺序和相邻分页边界。 */
import type {
  ConversationMessageData,
  ConversationMessageListData,
} from "@/api"

type MessageOrder = Pick<ConversationMessageData, "messageSeq">

/** 无损比较同一会话中持久消息的本地序号。 */
export function compareConversationMessages(left: MessageOrder, right: MessageOrder) {
  const a = BigInt(left.messageSeq)
  const b = BigInt(right.messageSeq)
  return a < b ? -1 : a > b ? 1 : 0
}

/** 合并已验证相邻的页面，空页仅更新本次读取方向的边界。 */
export function mergeConversationPage(
  current: ConversationMessageListData,
  page: ConversationMessageListData,
  direction: "before" | "after",
) {
  if (current.messages.length === 0) return page
  const messages = [
    ...new Map(
      [...current.messages, ...page.messages].map((message) => [
        message.id,
        message,
      ]),
    ).values(),
  ].sort(compareConversationMessages)
  return {
    latestAgentRun: page.latestAgentRun,
    pendingAgents: page.pendingAgents,
    messages,
    before:
      direction === "before" ? (page.before ?? current.before) : current.before,
    after:
      direction === "after" ? (page.after ?? current.after) : current.after,
    hasEarlier: direction === "before" ? page.hasEarlier : current.hasEarlier,
    hasLater: direction === "after" ? page.hasLater : current.hasLater,
  }
}
