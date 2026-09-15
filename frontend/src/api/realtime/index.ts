/** 按运行平台装配应用级成员实时事件流客户端。 */
import { Events } from "@wailsio/runtime"

import {
  ConnectRealtime,
  DisconnectRealtime,
} from "../../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  isApiError,
  normalizeError,
  requestMeta,
  responseError,
  sessionRequestMeta,
} from "@/api/client"
import { createNativeRealtimeTransport } from "@/api/realtime/native-transport"
import { RealtimeClient } from "@/api/realtime/realtime-client"
import { createWebRealtimeTransport } from "@/api/realtime/web-transport"
import { sessionPath } from "@/api/session"
import {
  currentSessionGeneration,
  subscribeSessionGeneration,
} from "@/api/session-scope"
import { resolveAppPlatform } from "@/platform/app-platform"

export type { RealtimeClientEvent, RealtimeState } from "@/api/realtime/realtime-client"

// 与 internal/appservice/types_realtime.go 中的原生端事件名保持一致。
const frameEventName = "cervi:realtime:frame"
const closedEventName = "cervi:realtime:closed"

/** 应用实例内唯一的成员实时事件流客户端。 */
export const realtimeClient = new RealtimeClient({
  transport:
    resolveAppPlatform() === "web"
      ? createWebRealtimeTransport({
          url: "/api/realtime",
          fetch: (url, init) => window.fetch(url, init),
          headers: () => {
            const meta = sessionRequestMeta()
            return meta
              ? {
                  Accept: "text/event-stream",
                  "Accept-Language": meta.locale,
                  Authorization: `Bearer ${meta.token}`,
                }
              : undefined
          },
          responseError,
        })
      : createNativeRealtimeTransport({
          connect: () => {
            const pending = ConnectRealtime(requestMeta())
            return Object.assign(
              pending.then(
                (connection) => connection.connectionId,
                (error: unknown) => {
                  throw normalizeError(error)
                },
              ),
              { cancel: () => void pending.cancel() },
            )
          },
          disconnect: async () => {
            await DisconnectRealtime(requestMeta())
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
  isSessionError: (error) =>
    isApiError(error) && sessionPath(error.state) !== null,
})
