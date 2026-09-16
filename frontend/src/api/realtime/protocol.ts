/** 实时 SSE 事件流的 JSON 事件契约与解码，与 internal/realtime/protocol 保持一致。 */
import type { SyncHeads } from "../../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

/** 当前协议主版本，只有破坏性演进才提升。 */
export const realtimeProtocolVersion = 1

const int64Max = 9223372036854775807n

/** 服务端经事件流下发的事件。 */
export type RealtimeServerFrame =
  | { type: "server_hello"; connectionId: string; syncHeads: SyncHeads }
  | { type: "visitor_hello"; connectionId: string }
  | { type: "ping" }
  | { type: "conversation_changed"; conversationId: string; version: bigint }
  | { type: "conversation_removed"; conversationId: string }
  | { type: "conversation_state_changed"; conversationId: string; version: bigint }
  | { type: "identity_profile_changed"; version: bigint }

/** 事件解码结果：未定义的事件种类忽略，主版本不一致与结构错误分别返回。 */
export type RealtimeServerFrameResult =
  | { status: "frame"; frame: RealtimeServerFrame }
  | { status: "ignored" }
  | { status: "unsupported_version" }
  | { status: "invalid"; reason: string }

type FrameData = Record<string, unknown>

/** 解码一条 SSE data 文本；先校验协议主版本，再校验事件种类、结构和字段类型。 */
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

/** 按事件种类读取事件数据，未定义的种类返回 undefined。 */
function decodeServerData(type: string, data: FrameData): RealtimeServerFrame | undefined {
  switch (type) {
    case "ping":
      return { type }
    case "server_hello": {
      // 探针校验和与身份资料版本是不透明比较值，保持字符串。
      const syncHeads = data.syncHeads
      if (!isFrameData(syncHeads) || !Number.isInteger(syncHeads.conversationCount)) {
        throw new Error("syncHeads is malformed")
      }
      return {
        type,
        connectionId: readString(data, "connectionId"),
        syncHeads: {
          conversationCount: syncHeads.conversationCount as number,
          conversationChecksum: readString(syncHeads, "conversationChecksum"),
          identityProfileVersion: readString(syncHeads, "identityProfileVersion"),
        },
      }
    }
    case "visitor_hello":
      return { type, connectionId: readString(data, "connectionId") }
    case "conversation_changed":
    case "conversation_state_changed":
      return { type, conversationId: readString(data, "conversationId"), version: readInt64(data, "version") }
    case "conversation_removed":
      return { type, conversationId: readString(data, "conversationId") }
    case "identity_profile_changed":
      return { type, version: readInt64(data, "version") }
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
