/** 按运行平台装配应用级成员实时事件流客户端。 */
import { Events } from "@wailsio/runtime"

import {
  ConnectAgentRunStream,
  ConnectRealtime,
  DisconnectAgentRunStream,
  DisconnectRealtime,
} from "../../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  isApiError,
  isNotFoundApiError,
  normalizeError,
  requestMeta,
  responseError,
  sessionRequestMeta,
} from "@/api/client"
import {
  createNativeRealtimeTransport,
  createNativeRunStreamTransport,
  type NativeRealtimeConnect,
} from "@/api/realtime/native-transport"
import { RealtimeClient } from "@/api/realtime/realtime-client"
import { RunStreamClient, type RunStreamErrorKind } from "@/api/realtime/run-stream"
import { createWebRealtimeTransport } from "@/api/realtime/web-transport"
import { sessionPath } from "@/api/session"
import {
  currentSessionGeneration,
  subscribeSessionGeneration,
} from "@/api/session-scope"
import { resolveAppPlatform } from "@/platform/app-platform"

export type { RealtimeClientEvent, RealtimeState } from "@/api/realtime/realtime-client"
export type { RunStreamEvent, RunStreamState } from "@/api/realtime/run-stream"
export type { RunStreamBlock, RunStreamPlanTask, RunStreamToolCall } from "@/api/realtime/protocol"

// 与 internal/appservice/types_realtime.go 中的原生端事件名保持一致。
const frameEventName = "cervi:realtime:frame"
const closedEventName = "cervi:realtime:closed"
const runFrameEventName = "cervi:realtime:run:frame"
const runClosedEventName = "cervi:realtime:run:closed"

/** 判断错误是否需要进入会话恢复入口。 */
function isSessionError(error: unknown) {
  return isApiError(error) && sessionPath(error.state) !== null
}

/** 判断运行过程流请求结束的原因；运行不存在或无权读取时不再重新请求。 */
function classifyRunStreamError(error: unknown): RunStreamErrorKind {
  if (isSessionError(error)) return "session"
  return isNotFoundApiError(error) ? "permanent" : "transient"
}

/** 返回事件流请求头；登录会话已变化时返回 undefined。 */
function streamHeaders() {
  const meta = sessionRequestMeta()
  return meta
    ? {
        Accept: "text/event-stream",
        "Accept-Language": meta.locale,
        Authorization: `Bearer ${meta.token}`,
      }
    : undefined
}

/** 包装可取消的原生连接调用，成功时返回本地流编号。 */
function nativeConnect(pending: ReturnType<typeof ConnectRealtime>): NativeRealtimeConnect {
  return Object.assign(
    pending.then(
      (connection) => connection.connectionId,
      (error: unknown) => {
        throw normalizeError(error)
      },
    ),
    { cancel: () => void pending.cancel() },
  )
}

/** 应用实例内唯一的成员实时事件流客户端。 */
export const realtimeClient = new RealtimeClient({
  transport:
    resolveAppPlatform() === "web"
      ? createWebRealtimeTransport({
          url: "/api/realtime",
          fetch: (url, init) => window.fetch(url, init),
          headers: streamHeaders,
          responseError,
        })
      : createNativeRealtimeTransport({
          connect: () => nativeConnect(ConnectRealtime(requestMeta())),
          disconnect: async (connectionId) => {
            await DisconnectRealtime(requestMeta(), connectionId)
          },
          onFrame: (listener) =>
            Events.On(frameEventName, (event) => {
              const data = event.data as { connectionId: string; frame: string }
              listener(data.connectionId, data.frame)
            }),
          onClosed: (listener) =>
            Events.On(closedEventName, (event) => {
              listener((event.data as { connectionId: string }).connectionId)
            }),
        }),
  generation: {
    current: currentSessionGeneration,
    subscribe: subscribeSessionGeneration,
  },
  isSessionError,
})

/** 创建指定运行的过程流客户端，调用方负责建立与关闭。 */
export function createRunStreamClient(runID: string) {
  return new RunStreamClient({
    transport:
      resolveAppPlatform() === "web"
        ? createWebRealtimeTransport({
            url: `/api/realtime/runs/${encodeURIComponent(runID)}`,
            fetch: (url, init) => window.fetch(url, init),
            headers: streamHeaders,
            responseError,
          })
        : createNativeRunStreamTransport(
            {
              connect: (id) => nativeConnect(ConnectAgentRunStream(requestMeta(), id)),
              disconnect: async (connectionID) => {
                await DisconnectAgentRunStream(requestMeta(), connectionID)
              },
              onFrame: (listener) =>
                Events.On(runFrameEventName, (event) => {
                  const data = event.data as { connectionId: string; runId: string; frame: string }
                  listener(data.connectionId, data.runId, data.frame)
                }),
              onClosed: (listener) =>
                Events.On(runClosedEventName, (event) => {
                  const data = event.data as { connectionId: string; runId: string }
                  listener(data.connectionId, data.runId)
                }),
            },
            runID,
          ),
    classifyError: classifyRunStreamError,
  })
}
