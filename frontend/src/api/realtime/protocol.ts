/** 实时 SSE 事件流的 JSON 事件契约与解码，与 internal/realtime/protocol 保持一致。 */
import type {
  AgentRunBlockKind,
  AgentToolCallStatus,
  SyncHeads,
} from "../../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

/** 运行过程流内容块的已知类型，取值与 AgentRunBlockKind 一致。 */
const blockKinds = new Set(["thinking", "content", "tool_call"])

/** 运行过程流工具调用的已知状态，取值与 AgentToolCallStatus 一致。 */
const toolCallStatuses = new Set(["queued", "running", "succeeded", "failed"])

/** 当前协议主版本，只有破坏性演进才提升。 */
export const realtimeProtocolVersion = 1

const int64Max = 9223372036854775807n

/** 运行过程流中的工具调用名称与状态，完整参数与结果经过程详情查询读取。 */
export type RunStreamToolCall = {
  name: string
  status: AgentToolCallStatus
  startedAt?: string
  completedAt?: string
}

/** 运行过程流中按位置排列的展示内容块。 */
export type RunStreamBlock = {
  id: string
  position: bigint
  kind: AgentRunBlockKind
  text: string
  toolCall?: RunStreamToolCall
}

/** 可按顺序应用到运行过程流快照的一条变更。 */
export type RunStreamOperation =
  | { kind: "upsert_block"; block: RunStreamBlock }
  | { kind: "append_block_text"; blockId: string; text: string }
  | { kind: "remove_blocks"; blockIds: string[] }
  | { kind: "append_candidate"; text: string }
  | { kind: "clear_candidate" }
  | { kind: "reset" }

/** 服务端经事件流下发的事件。 */
export type RealtimeServerFrame =
  | { type: "server_hello"; connectionId: string; syncHeads: SyncHeads }
  | { type: "visitor_hello"; connectionId: string }
  | { type: "ping" }
  | { type: "conversation_changed"; conversationId: string; version: bigint }
  | { type: "conversation_removed"; conversationId: string }
  | { type: "conversation_state_changed"; conversationId: string; version: bigint }
  | { type: "identity_profile_changed"; version: bigint }
  | {
      type: "run_stream_snapshot"
      runId: string
      streamId: string
      attempt: number
      sequence: bigint
      part: number
      partCount: number
      candidateContent: string
      blocks: RunStreamBlock[]
    }
  | {
      type: "run_stream_delta"
      runId: string
      streamId: string
      attempt: number
      baseSequence: bigint
      sequence: bigint
      operations: RunStreamOperation[]
    }
  | { type: "run_stream_ended"; runId: string }

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
    case "run_stream_snapshot":
      return {
        type,
        runId: readString(data, "runId"),
        streamId: readString(data, "streamId"),
        attempt: readInt(data, "attempt"),
        sequence: readInt64(data, "sequence"),
        part: readInt(data, "part"),
        partCount: readInt(data, "partCount"),
        candidateContent: typeof data.candidateContent === "string" ? data.candidateContent : "",
        blocks: readArray(data, "blocks").map(readBlock),
      }
    case "run_stream_delta":
      return {
        type,
        runId: readString(data, "runId"),
        streamId: readString(data, "streamId"),
        attempt: readInt(data, "attempt"),
        baseSequence: readInt64(data, "baseSequence"),
        sequence: readInt64(data, "sequence"),
        operations: readArray(data, "operations").map(readOperation),
      }
    case "run_stream_ended":
      return { type, runId: readString(data, "runId") }
    default:
      return undefined
  }
}

/** 读取一个运行过程流内容块，枚举值不在契约内时报错。 */
function readBlock(value: unknown): RunStreamBlock {
  if (!isFrameData(value)) {
    throw new Error("block is not an object")
  }
  const call = value.toolCall
  return {
    id: readString(value, "id"),
    position: readInt64(value, "position"),
    kind: readEnum(value, "kind", blockKinds) as AgentRunBlockKind,
    text: typeof value.text === "string" ? value.text : "",
    toolCall: isFrameData(call)
      ? {
          name: readString(call, "name"),
          status: readEnum(call, "status", toolCallStatuses) as AgentToolCallStatus,
          startedAt: typeof call.startedAt === "string" ? call.startedAt : undefined,
          completedAt: typeof call.completedAt === "string" ? call.completedAt : undefined,
        }
      : undefined,
  }
}

/** 读取一条增量操作，未定义的操作类型报错。 */
function readOperation(value: unknown): RunStreamOperation {
  if (!isFrameData(value)) {
    throw new Error("operation is not an object")
  }
  const kind = readString(value, "kind")
  switch (kind) {
    case "upsert_block":
      return { kind, block: readBlock(value.block) }
    case "append_block_text":
      return { kind, blockId: readString(value, "blockId"), text: readString(value, "text") }
    case "remove_blocks":
      return { kind, blockIds: readArray(value, "blockIds").map((id) => {
        if (typeof id !== "string") throw new Error("blockIds contains a non-string")
        return id
      }) }
    case "append_candidate":
      return { kind, text: readString(value, "text") }
    case "clear_candidate":
    case "reset":
      return { kind }
    default:
      throw new Error(`unsupported run stream operation ${kind}`)
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

/** 读取数组字段，缺少字段时返回空数组。 */
function readArray(data: FrameData, key: string): unknown[] {
  const value = data[key]
  if (value === undefined || value === null) {
    return []
  }
  if (!Array.isArray(value)) {
    throw new Error(`${key} is not an array`)
  }
  return value
}

/** 读取非负整数字段。 */
function readInt(data: FrameData, key: string): number {
  const value = data[key]
  if (typeof value !== "number" || !Number.isInteger(value) || value < 0) {
    throw new Error(`${key} is not a non-negative integer`)
  }
  return value
}

/** 读取取值在已知集合内的字符串字段。 */
function readEnum(data: FrameData, key: string, values: Set<string>): string {
  const value = readString(data, key)
  if (!values.has(value)) {
    throw new Error(`${key} is not a known ${key} value`)
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
