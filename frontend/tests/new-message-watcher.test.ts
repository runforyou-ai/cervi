/** 用可控的权威读取验证新消息通知的实际新增范围、静音与提及策略及追赶边界。 */
import assert from "node:assert/strict"
import { test } from "node:test"
import { NewMessageWatcher } from "../src/features/notifications/new-message-watcher.ts"
import type { ConversationMessage, InboxConversation } from "../src/api/index.ts"

/** 构造带未读、静音和末条位置的会话行。 */
function conversation(id: string, overrides: Partial<InboxConversation> = {}): InboxConversation {
  return {
    id,
    unreadCount: 1,
    mentionedUnreadCount: 0,
    markedUnread: false,
    muted: false,
    lastMessageId: null,
    group: null,
    ...overrides,
  } as InboxConversation
}

/** 构造他人发送的普通文本消息。 */
function message(id: string, overrides: Partial<ConversationMessage> = {}): ConversationMessage {
  return {
    id,
    body: `正文 ${id}`,
    systemEvent: null,
    attachment: null,
    sender: { sourceId: "other", displayName: "同事" },
    ...overrides,
  } as ConversationMessage
}

/** 等待合并窗口到期与串行处理链执行完毕。 */
async function settle() {
  for (let round = 0; round < 12; round += 1) {
    await new Promise((resolve) => setTimeout(resolve, 1))
  }
}

/** 创建使用内存权威数据的观察器与投递记录。 */
function fixture() {
  const delivered: string[] = []
  const failures: unknown[] = []
  const conversations = new Map<string, InboxConversation | null>()
  const messages = new Map<string, ConversationMessage[]>()
  const mentions = new Map<string, string[]>()
  const reads: string[] = []
  const watcher = new NewMessageWatcher(
    "me",
    {
      readConversations: async () =>
        [...conversations.values()].filter((row): row is InboxConversation => row !== null),
      readConversation: async (conversationId) => {
        reads.push(conversationId)
        return conversations.get(conversationId) ?? null
      },
      readMessages: async (conversationId) => messages.get(conversationId) ?? [],
      readPendingMentions: async (conversationId) => mentions.get(conversationId) ?? [],
      deliver: async (row, item) => {
        delivered.push(`${row.id}:${item.id}`)
      },
      failed: (error) => failures.push(error),
    },
    { mergeWindowMs: 1 },
  )
  return { watcher, delivered, failures, conversations, messages, mentions, reads }
}

test("同一会话连续到达的多条消息逐条判断策略，重复通知不再投递", async () => {
  const f = fixture()
  f.conversations.set("c1", conversation("c1", { lastMessageId: "m1", unreadCount: 0 }))
  f.messages.set("c1", [message("m1")])
  f.watcher.receive({ type: "server_hello", connectionId: "1", syncHeads: {} as never })
  await settle()

  f.conversations.set("c1", conversation("c1", { lastMessageId: "m3", unreadCount: 2 }))
  f.messages.set("c1", [message("m1"), message("m2"), message("m3")])
  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 2n })
  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 3n })
  await settle()
  assert.deepEqual(f.delivered, ["c1:m2", "c1:m3"])

  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 4n })
  await settle()
  assert.deepEqual(f.delivered, ["c1:m2", "c1:m3"])
  f.watcher.dispose()
})

test("静音群聊只提醒 @ 本人的消息，静音单聊不提醒", async () => {
  const f = fixture()
  f.conversations.set("c1", conversation("c1", { lastMessageId: "m1", unreadCount: 0, muted: true, group: {} as never }))
  f.conversations.set("c2", conversation("c2", { lastMessageId: "n1", unreadCount: 0, muted: true }))
  f.messages.set("c1", [message("m1")])
  f.messages.set("c2", [message("n1")])
  f.watcher.receive({ type: "server_hello", connectionId: "1", syncHeads: {} as never })
  await settle()

  f.conversations.set("c1", conversation("c1", { lastMessageId: "m3", unreadCount: 2, muted: true, group: {} as never }))
  f.messages.set("c1", [message("m1"), message("m2"), message("m3")])
  f.mentions.set("c1", ["m3"])
  f.conversations.set("c2", conversation("c2", { lastMessageId: "n2", unreadCount: 1, muted: true }))
  f.messages.set("c2", [message("n1"), message("n2")])
  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 2n })
  f.watcher.receive({ type: "conversation_changed", conversationId: "c2", version: 2n })
  await settle()
  assert.deepEqual(f.delivered, ["c1:m3"])
  f.watcher.dispose()
})

test("重连追赶的历史只更新基线，之后的实时消息照常提醒", async () => {
  const f = fixture()
  f.conversations.set("c1", conversation("c1", { lastMessageId: "m1", unreadCount: 0 }))
  f.messages.set("c1", [message("m1")])
  f.watcher.receive({ type: "server_hello", connectionId: "1", syncHeads: {} as never })
  await settle()

  // 断线期间积压四条消息，重连问候与随后的变更通知都不逐条补弹。
  f.conversations.set("c1", conversation("c1", { lastMessageId: "m5", unreadCount: 4 }))
  f.messages.set("c1", ["m1", "m2", "m3", "m4", "m5"].map((id) => message(id)))
  f.watcher.receive({ type: "server_hello", connectionId: "2", syncHeads: {} as never })
  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 5n })
  await settle()
  assert.deepEqual(f.delivered, [])

  f.conversations.set("c1", conversation("c1", { lastMessageId: "m6", unreadCount: 5 }))
  f.messages.set("c1", ["m1", "m2", "m3", "m4", "m5", "m6"].map((id) => message(id)))
  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 6n })
  await settle()
  assert.deepEqual(f.delivered, ["c1:m6"])
  f.watcher.dispose()
})

test("他端已读、本人消息与空正文的状态消息只更新未读", async () => {
  const f = fixture()
  f.conversations.set("c1", conversation("c1", { lastMessageId: "m1", unreadCount: 2 }))
  f.conversations.set("c2", conversation("c2", { lastMessageId: "n1", unreadCount: 0 }))
  f.messages.set("c1", [message("m1")])
  f.messages.set("c2", [message("n1")])
  f.watcher.receive({ type: "server_hello", connectionId: "1", syncHeads: {} as never })
  await settle()

  // 他端已读只降低未读，本人在另一端发送与 AI 运行失败消息都不提醒。
  f.conversations.set("c1", conversation("c1", { lastMessageId: "m2", unreadCount: 0 }))
  f.messages.set("c1", [message("m1"), message("m2")])
  f.conversations.set("c2", conversation("c2", { lastMessageId: "n3", unreadCount: 2 }))
  f.messages.set("c2", [
    message("n1"),
    message("n2", { sender: { sourceId: "me", displayName: "我" } as never }),
    message("n3", { body: "" }),
  ])
  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 2n })
  f.watcher.receive({ type: "conversation_changed", conversationId: "c2", version: 2n })
  await settle()
  assert.deepEqual(f.delivered, [])
  f.watcher.dispose()
})

test("基线缺失的会话只按最新一条判断，失权会话不再提醒", async () => {
  const f = fixture()
  f.watcher.receive({ type: "server_hello", connectionId: "1", syncHeads: {} as never })
  await settle()

  // 会话在本次基线之外，只有最新一条参与策略判断。
  f.conversations.set("c9", conversation("c9", { lastMessageId: "m5", unreadCount: 3 }))
  f.messages.set("c9", ["m1", "m2", "m3", "m4", "m5"].map((id) => message(id)))
  f.watcher.receive({ type: "conversation_changed", conversationId: "c9", version: 2n })
  await settle()
  assert.deepEqual(f.delivered, ["c9:m5"])

  f.conversations.set("c9", null)
  f.watcher.receive({ type: "conversation_changed", conversationId: "c9", version: 3n })
  await settle()
  assert.deepEqual(f.delivered, ["c9:m5"])
  assert.deepEqual(f.failures, [])
  f.watcher.dispose()
})

test("订阅时事件流已建立且没有问候事件，首次变化先取基线", async () => {
  const f = fixture()
  f.conversations.set("c1", conversation("c1", { lastMessageId: "m3", unreadCount: 3 }))
  f.messages.set("c1", ["m1", "m2", "m3"].map((id) => message(id)))
  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 3n })
  await settle()
  assert.deepEqual(f.delivered, [])

  f.conversations.set("c1", conversation("c1", { lastMessageId: "m4", unreadCount: 4 }))
  f.messages.set("c1", ["m1", "m2", "m3", "m4"].map((id) => message(id)))
  f.watcher.receive({ type: "conversation_changed", conversationId: "c1", version: 4n })
  await settle()
  assert.deepEqual(f.delivered, ["c1:m4"])
  f.watcher.dispose()
})
