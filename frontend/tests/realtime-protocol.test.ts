/** 按 Go 与 TypeScript 共用的事件夹具校验前端事件解码。 */
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { test } from "node:test"
import { decodeServerFrame, type RealtimeServerFrame } from "../src/api/realtime/protocol.ts"

type FixtureCase = {
  name: string
  result: "frame" | "ignored" | "unsupported_version" | "invalid"
  wire: unknown
}

const fixtures = JSON.parse(
  readFileSync(new URL("../../internal/realtime/protocol/testdata/frames.json", import.meta.url), "utf8"),
) as FixtureCase[]

const conversationId = "0190f5a2-7c1e-7d3a-9b2f-3c4d5e6f7a8b"

const expectedFrames: Record<string, RealtimeServerFrame> = {
  server_hello: {
    type: "server_hello",
    connectionId: "conn-01",
    syncHeads: { conversationCount: 3, conversationChecksum: "18446744073709551615", identityProfileVersion: "9223372036854775807" },
  },
  visitor_hello: { type: "visitor_hello", connectionId: "conn-02" },
  ping: { type: "ping" },
  conversation_changed: { type: "conversation_changed", conversationId, version: 9223372036854775807n },
  conversation_state_changed: { type: "conversation_state_changed", conversationId, version: 42n },
  identity_profile_changed: { type: "identity_profile_changed", version: 9007199254740993n },
  conversation_removed: { type: "conversation_removed", conversationId },
  conversation_changed_extra_fields: { type: "conversation_changed", conversationId, version: 7n },
  ping_without_data: { type: "ping" },
}

test("事件按共用夹具解码，64 位版本不丢精度", () => {
  assert.ok(fixtures.length > 0)
  for (const fixture of fixtures) {
    const result = decodeServerFrame(JSON.stringify(fixture.wire))
    assert.equal(result.status, fixture.result, fixture.name)
    if (result.status === "frame") {
      assert.deepEqual(result.frame, expectedFrames[fixture.name], fixture.name)
    }
  }
})

test("非 JSON 文本按结构错误返回", () => {
  assert.equal(decodeServerFrame("not json").status, "invalid")
})
