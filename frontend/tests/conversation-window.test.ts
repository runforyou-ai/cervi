/** 验证服务端消息精度和连续分页的浏览边界。 */
import assert from "node:assert/strict"
import { test } from "node:test"
import {
  compareConversationMessages,
  mergeConversationPage,
} from "../src/features/inbox/conversation-window.ts"
import type {
  ConversationMessageData,
  ConversationMessageListData,
} from "../src/api/inbox.ts"

/** 构造具有真实传输字段的消息。 */
function message(
  id: string,
  sequence: string | null,
  originatedAt = "2026-09-05T00:00:00Z",
): ConversationMessageData {
  return {
    id,
    groupMessageSequence: sequence,
    originatedAt,
    sourceOrder: 0,
  } as ConversationMessageData
}

/** 构造带两个端点的连续页面。 */
function page(
  messages: ConversationMessageData[],
  hasEarlier = true,
  hasLater = true,
): ConversationMessageListData {
  return {
    messages,
    latestAgentRun: null,
    before: messages[0]?.id ?? null,
    after: messages[messages.length - 1]?.id ?? null,
    hasEarlier,
    hasLater,
  }
}

test("群聊使用超过 JavaScript 安全整数的服务端序号", () => {
  const first = message("z", "9007199254740992", "2026-09-06T00:00:00Z")
  const second = message("a", "9007199254740993", "2026-09-05T00:00:00Z")
  assert.equal(compareConversationMessages(first, second), -1)
})

test("没有新消息的页面仍更新运行终态并保留既有消息", () => {
  const current = page([message("reply", null)], false, false)
  const incoming = page([], false, false)
  incoming.latestAgentRun = {
    id: "run", agentName: "AI 助手", status: "failed", errorCode: null, lastError: "model rejected input",
  } as NonNullable<ConversationMessageListData["latestAgentRun"]>
  const merged = mergeConversationPage(current, incoming, "after")
  assert.equal(merged.latestAgentRun, incoming.latestAgentRun)
  assert.deepEqual(merged.messages, current.messages)
  assert.equal(merged.after, current.after)
})

test("单聊和客服保留微秒时间、来源序号和 ID 的排序", () => {
  const first = message("z", null, "2026-09-05T00:00:00.123001Z")
  const second = message("a", null, "2026-09-05T00:00:00.123002Z")
  assert.ok(compareConversationMessages(first, second) < 0)
  assert.ok(
    compareConversationMessages(
      { ...first, sourceOrder: 1 },
      { ...first, sourceOrder: 2 },
    ) < 0,
  )
  assert.ok(
    compareConversationMessages({ ...first, id: "a" }, { ...first, id: "b" }) <
      0,
  )
})

test("历史页前插与后续页追加各自保留另一端边界", () => {
  const current = page([message("b", "2"), message("c", "3")])
  const earlier = mergeConversationPage(
    current,
    page([message("a", "1")], false),
    "before",
  )
  assert.deepEqual(
    earlier.messages.map((row) => row.id),
    ["a", "b", "c"],
  )
  assert.deepEqual(
    [earlier.before, earlier.after, earlier.hasEarlier, earlier.hasLater],
    ["a", "c", false, true],
  )
  const later = mergeConversationPage(
    earlier,
    page([message("d", "4")], true, false),
    "after",
  )
  assert.deepEqual(
    [later.before, later.after, later.hasEarlier, later.hasLater],
    ["a", "d", false, false],
  )
})

test("空轮询不会清掉端点，重复页不会重复消息", () => {
  const current = page([message("b", "2"), message("c", "3")])
  const empty = mergeConversationPage(current, page([], false, false), "after")
  assert.deepEqual(
    [empty.before, empty.after, empty.hasEarlier, empty.hasLater],
    ["b", "c", true, false],
  )
  const duplicate = mergeConversationPage(
    empty,
    page([message("c", "3"), message("d", "4")], true, false),
    "after",
  )
  assert.deepEqual(
    duplicate.messages.map((row) => row.id),
    ["b", "c", "d"],
  )
})
