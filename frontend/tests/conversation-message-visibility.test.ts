/** 验证可视提及的正文阈值及视口上下边界。 */
import assert from "node:assert/strict"
import { test } from "node:test"
import { isConversationMessageVisible } from "../src/features/inbox/conversation-message-visibility.ts"

test("完整可见和达到正文阈值的消息视为已看到", () => {
  const viewport = { top: 100, bottom: 500, height: 400 }
  assert.equal(isConversationMessageVisible({ top: 200, bottom: 240, height: 40 }, viewport), true)
  assert.equal(isConversationMessageVisible({ top: 480, bottom: 520, height: 40 }, viewport), true)
  assert.equal(isConversationMessageVisible({ top: 468, bottom: 1468, height: 1000 }, viewport), true)
})

test("视口外、只露出少量正文和隐藏消息不确认", () => {
  const viewport = { top: 100, bottom: 500, height: 400 }
  for (const row of [
    { top: 20, bottom: 80, height: 60 },
    { top: 500, bottom: 540, height: 40 },
    { top: 481, bottom: 521, height: 40 },
    { top: 0, bottom: 0, height: 0 },
  ]) assert.equal(isConversationMessageVisible(row, viewport), false)
  assert.equal(isConversationMessageVisible({ top: 0, bottom: 40, height: 40 }, { top: 0, bottom: 0, height: 0 }), false)
})
