/** 按 Go 与 TypeScript 共用的帧夹具校验前端客户端帧编码与服务端帧解码。 */
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { test } from "node:test"
import {
  decodeServerFrame,
  encodeClientFrame,
  type RealtimeClientFrame,
  type RealtimeServerFrame,
} from "../src/api/realtime/protocol.ts"

type FixtureCase = {
  name: string
  direction: "client" | "server"
  result: "frame" | "ignored" | "unsupported_version" | "invalid"
  encode?: boolean
  wire: unknown
}

const fixtures = JSON.parse(
  readFileSync(new URL("../../internal/realtime/protocol/testdata/frames.json", import.meta.url), "utf8"),
) as FixtureCase[]

const conversationId = "0190f5a2-7c1e-7d3a-9b2f-3c4d5e6f7a8b"

const clientFrames: Record<string, RealtimeClientFrame> = {
  authenticate: { type: "authenticate", token: "cervi-token-example" },
  client_hello: { type: "client_hello", clientKind: "web", appVersion: "0.1.0" },
  client_hello_capabilities: { type: "client_hello", clientKind: "desktop", appVersion: "0.1.0", capabilities: ["future_capability"] },
  client_ping: { type: "ping" },
  client_pong: { type: "pong" },
}

const serverFrames: Record<string, RealtimeServerFrame> = {
  authenticated: { type: "authenticated" },
  server_hello: {
    type: "server_hello",
    connectionId: "conn-01",
    capabilities: [],
    syncHeads: { conversationCount: 3, conversationChecksum: "18446744073709551615", identityProfileVersion: "9223372036854775807" },
  },
  server_ping: { type: "ping" },
  server_pong: { type: "pong" },
  conversation_changed: { type: "conversation_changed", conversationId, version: 9223372036854775807n },
  conversation_state_changed: { type: "conversation_state_changed", conversationId, version: 42n },
  identity_profile_changed: { type: "identity_profile_changed", version: 9007199254740993n },
  access_revoked: { type: "access_revoked", conversationId },
  session_revoked: { type: "session_revoked", reason: "logout" },
  session_revoked_unknown_reason: { type: "session_revoked", reason: "future_reason" },
  server_going_away: { type: "server_going_away" },
  realtime_error: { type: "realtime_error", code: "unsupported_version" },
  conversation_changed_extra_fields: { type: "conversation_changed", conversationId, version: 7n },
  server_ping_without_data: { type: "ping" },
}

test("客户端帧编码与共用夹具的线上格式一致", () => {
  const cases = fixtures.filter((fixture) => fixture.direction === "client" && fixture.encode)
  assert.ok(cases.length > 0)
  for (const fixture of cases) {
    const frame = clientFrames[fixture.name]
    assert.ok(frame, `missing expected client frame ${fixture.name}`)
    assert.deepEqual(JSON.parse(encodeClientFrame(frame)), fixture.wire, fixture.name)
  }
})

test("服务端帧按共用夹具解码，64 位版本不丢精度", () => {
  const cases = fixtures.filter((fixture) => fixture.direction === "server")
  assert.ok(cases.length > 0)
  for (const fixture of cases) {
    const result = decodeServerFrame(JSON.stringify(fixture.wire))
    assert.equal(result.status, fixture.result, fixture.name)
    if (result.status === "frame") {
      assert.deepEqual(result.frame, serverFrames[fixture.name], fixture.name)
    }
  }
})

test("非 JSON 文本按结构错误返回", () => {
  assert.equal(decodeServerFrame("not json").status, "invalid")
})
