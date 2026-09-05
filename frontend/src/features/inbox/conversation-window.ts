/** 按服务端精度维护消息顺序和相邻分页边界。 */
import type {
  ConversationMessageData,
  ConversationMessageListData,
} from "@/api"

type MessageOrder = Pick<
  ConversationMessageData,
  "id" | "originatedAt" | "sourceOrder" | "groupMessageSequence"
>

/** 无损比较群序号，其他消息保留来源时间的小数秒精度。 */
export function compareConversationMessages(
  left: MessageOrder,
  right: MessageOrder,
) {
  if (
    left.groupMessageSequence !== null &&
    right.groupMessageSequence !== null
  ) {
    const a = BigInt(left.groupMessageSequence)
    const b = BigInt(right.groupMessageSequence)
    return a < b ? -1 : a > b ? 1 : 0
  }
  const milliseconds =
    Date.parse(left.originatedAt) - Date.parse(right.originatedAt)
  if (milliseconds) return milliseconds
  // 服务端时间精度高于 Date，毫秒相同时继续比较纳秒的小数部分。
  const a = (left.originatedAt.match(/\.(\d+)/)?.[1] ?? "").padEnd(9, "0")
  const b = (right.originatedAt.match(/\.(\d+)/)?.[1] ?? "").padEnd(9, "0")
  return (
    a.localeCompare(b) ||
    left.sourceOrder - right.sourceOrder ||
    left.id.localeCompare(right.id)
  )
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
    messages,
    before:
      direction === "before" ? (page.before ?? current.before) : current.before,
    after:
      direction === "after" ? (page.after ?? current.after) : current.after,
    hasEarlier: direction === "before" ? page.hasEarlier : current.hasEarlier,
    hasLater: direction === "after" ? page.hasLater : current.hasLater,
  }
}
