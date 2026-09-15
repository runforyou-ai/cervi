/** 实时 WebSocket JSON 帧契约与编解码，与 internal/realtime/protocol 保持一致。 */
import type { SyncHeads } from "../../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

/** 当前协议主版本，只有破坏性演进才提升。 */
export const realtimeProtocolVersion = 1

const int64Max = 9223372036854775807n

/** 发起连接的客户端种类。 */
export type RealtimeClientKind = "web" | "desktop" | "mobile" | "messenger"

/** 客户端发往服务端的帧。 */
export type RealtimeClientFrame =
  | { type: "authenticate"; token: string }
  | { type: "client_hello"; clientKind: RealtimeClientKind; appVersion: string; capabilities?: string[] }
  | { type: "ping" }
  | { type: "pong" }

/** 服务端发往客户端的帧；未定义的 reason 与 code 取值原样保留。 */
export type RealtimeServerFrame =
  | { type: "authenticated" }
  | { type: "server_hello"; connectionId: string; capabilities: string[]; syncHeads: SyncHeads }
  | { type: "ping" }
  | { type: "pong" }
  | { type: "conversation_changed"; conversationId: string; version: bigint }
  | { type: "conversation_state_changed"; conversationId: string; version: bigint }
  | { type: "identity_profile_changed"; version: bigint }
  | { type: "access_revoked"; conversationId: string }
  | { type: "session_revoked"; reason: string }
  | { type: "server_going_away" }
  | { type: "realtime_error"; code: string }

/** 服务端帧解码结果：未定义的帧种类忽略，主版本不一致与结构错误分别返回。 */
export type RealtimeServerFrameResult =
  | { status: "frame"; frame: RealtimeServerFrame }
  | { status: "ignored" }
  | { status: "unsupported_version" }
  | { status: "invalid"; reason: string }

type FrameData = Record<string, unknown>

/** 把客户端帧编码为带协议主版本的 JSON 文本。 */
export function encodeClientFrame(frame: RealtimeClientFrame): string {
  const { type, ...data } = frame
  return JSON.stringify({ v: realtimeProtocolVersion, type, data })
}

/** 解码服务端帧；先校验协议主版本，再校验帧种类、结构和字段类型。 */
export function decodeServerFrame(text: string): RealtimeServerFrameResult {
  try {
    const value: unknown = JSON.parse(text)
    if (!isFrameData(value) || (value.v !== undefined && typeof value.v !== "number")) {
      return { status: "invalid", reason: "frame envelope is malformed" }
    }
    if (value.v !== realtimeProtocolVersion) {
      return { status: "unsupported_version" }
    }
    const data = value.data ?? {}
    if (typeof value.type !== "string" || !isFrameData(data)) {
      return { status: "invalid", reason: "frame type or data is malformed" }
    }
    const frame = decodeServerData(value.type, data)
    return frame ? { status: "frame", frame } : { status: "ignored" }
  } catch (error) {
    return { status: "invalid", reason: error instanceof Error ? error.message : String(error) }
  }
}

/** 按帧种类读取服务端帧数据，未定义的种类返回 undefined。 */
function decodeServerData(type: string, data: FrameData): RealtimeServerFrame | undefined {
  switch (type) {
    case "authenticated":
    case "ping":
    case "pong":
    case "server_going_away":
      return { type }
    case "server_hello": {
      // 能力集合省略时为空数组；探针校验和与身份资料版本是不透明比较值，保持字符串。
      const syncHeads = data.syncHeads
      const capabilities = data.capabilities ?? []
      if (!isFrameData(syncHeads) || !Number.isInteger(syncHeads.conversationCount)) {
        throw new Error("syncHeads is malformed")
      }
      if (!Array.isArray(capabilities) || capabilities.some((item) => typeof item !== "string")) {
        throw new Error("capabilities is not a string array")
      }
      return {
        type,
        connectionId: readString(data, "connectionId"),
        capabilities,
        syncHeads: {
          conversationCount: syncHeads.conversationCount as number,
          conversationChecksum: readString(syncHeads, "conversationChecksum"),
          identityProfileVersion: readString(syncHeads, "identityProfileVersion"),
        },
      }
    }
    case "conversation_changed":
    case "conversation_state_changed":
      return { type, conversationId: readString(data, "conversationId"), version: readInt64(data, "version") }
    case "identity_profile_changed":
      return { type, version: readInt64(data, "version") }
    case "access_revoked":
      return { type, conversationId: readString(data, "conversationId") }
    case "session_revoked":
      return { type, reason: readString(data, "reason") }
    case "realtime_error":
      return { type, code: readString(data, "code") }
    default:
      return undefined
  }
}

/** 判断值是否为 JSON 对象。 */
function isFrameData(value: unknown): value is FrameData {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

/** 读取字符串字段。 */
function readString(data: FrameData, key: string): string {
  const value = data[key]
  if (typeof value !== "string") {
    throw new Error(`${key} is not a string`)
  }
  return value
}

/** 读取以十进制字符串传输的非负 64 位整数。 */
function readInt64(data: FrameData, key: string): bigint {
  const value = data[key]
  if (typeof value !== "string" || !/^(0|[1-9]\d*)$/.test(value) || BigInt(value) > int64Max) {
    throw new Error(`${key} is not an int64 string`)
  }
  return BigInt(value)
}
