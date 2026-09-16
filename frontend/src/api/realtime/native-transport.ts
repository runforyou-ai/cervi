/** 原生端成员事件流传输：串行执行 Go 侧连接与断开，按连接编号投递事件。 */
import type { RealtimeTransport } from "./realtime-client.ts"

/** 可取消的原生连接调用，成功时返回本地连接编号。 */
export type NativeRealtimeConnect = Promise<string> & { cancel: () => void }

/** Go 侧事件流的连接、断开与事件监听。 */
export type NativeRealtimeBridge = {
  connect: () => NativeRealtimeConnect
  disconnect: () => Promise<void>
  onFrame: (listener: (connectionId: string, frame: string) => void) => () => void
  onClosed: (listener: (connectionId: string) => void) => () => void
}

type NativeEvent = { connectionId: string; frame?: string }

/** 创建原生端事件流传输；Go 侧只保留一条当前事件流，连接与断开按调用顺序串行执行。 */
export function createNativeRealtimeTransport(bridge: NativeRealtimeBridge): RealtimeTransport {
  // 上一次连接请求及其清理完成后才发起下一次连接，旧请求晚到时不会替换新事件流。
  let operations: Promise<void> = Promise.resolve()

  return {
    open(handlers) {
      let closed = false
      let connectionId: string | undefined
      let pending: NativeRealtimeConnect | undefined
      // 连接编号返回前到达的事件先缓存，拿到编号后只回放属于本连接的事件。
      const early: NativeEvent[] = []

      /** 投递属于本连接的事件，事件流结束时释放监听。 */
      function deliver(event: NativeEvent) {
        if (closed) return
        if (connectionId === undefined) {
          early.push(event)
          return
        }
        if (event.connectionId !== connectionId) return
        if (event.frame !== undefined) {
          handlers.frame(event.frame)
          return
        }
        release()
        handlers.closed()
      }

      const stopFrame = bridge.onFrame((id, frame) => deliver({ connectionId: id, frame }))
      const stopClosed = bridge.onClosed((id) => deliver({ connectionId: id }))

      /** 停止接收事件并注销监听。 */
      function release() {
        closed = true
        stopFrame()
        stopClosed()
      }

      operations = operations
        .then(async () => {
          if (closed) return
          pending = bridge.connect()
          let id: string
          try {
            id = await pending
          } catch (error) {
            if (!closed) {
              release()
              handlers.closed(error)
            }
            return
          } finally {
            pending = undefined
          }
          if (closed) {
            // 调用方已关闭时连接可能仍已建立，断开后才允许下一次连接。
            await bridge.disconnect().catch((error: unknown) => console.warn("断开过期的实时事件流失败", error))
            return
          }
          connectionId = id
          for (const event of early.splice(0)) deliver(event)
        })
        .catch((error: unknown) => console.error("处理原生实时事件流连接失败", error))

      return () => {
        if (closed) return
        const established = connectionId !== undefined
        release()
        pending?.cancel()
        if (established) {
          operations = operations.then(() =>
            bridge.disconnect().catch((error: unknown) => console.warn("断开实时事件流失败", error)),
          )
        }
      }
    },
  }
}

/** Go 侧运行过程流的连接、断开与事件监听；事件携带运行编号，取得本地流编号前据此过滤。 */
export type NativeRunStreamBridge = {
  connect: (runId: string) => NativeRealtimeConnect
  disconnect: (connectionId: string) => Promise<void>
  onFrame: (listener: (connectionId: string, runId: string, frame: string) => void) => () => void
  onClosed: (listener: (connectionId: string, runId: string) => void) => () => void
}

/** 创建原生端运行过程流传输；每条流独立建立与关闭，按本地流编号分流事件。 */
export function createNativeRunStreamTransport(bridge: NativeRunStreamBridge, runId: string): RealtimeTransport {
  return {
    open(handlers) {
      let closed = false
      let connectionId: string | undefined
      // 连接编号返回前到达的事件先缓存，拿到编号后只回放属于本条流的事件。
      const early: NativeEvent[] = []

      /** 投递属于本条流的事件，流结束时释放监听。 */
      function deliver(event: NativeEvent) {
        if (closed) return
        if (connectionId === undefined) {
          early.push(event)
          return
        }
        if (event.connectionId !== connectionId) return
        if (event.frame !== undefined) {
          handlers.frame(event.frame)
          return
        }
        release()
        handlers.closed()
      }

      // 并发的运行过程流共用同一组 Wails 事件，取得本地流编号前先按运行编号过滤。
      const stopFrame = bridge.onFrame((id, run, frame) => {
        if (run === runId) deliver({ connectionId: id, frame })
      })
      const stopClosed = bridge.onClosed((id, run) => {
        if (run === runId) deliver({ connectionId: id })
      })

      /** 停止接收事件并注销监听。 */
      function release() {
        closed = true
        stopFrame()
        stopClosed()
      }

      const pending = bridge.connect(runId)
      void pending.then(
        (id) => {
          if (closed) {
            // 调用方已关闭时流可能仍已建立，建立后立即断开。
            void bridge.disconnect(id).catch((error: unknown) => console.warn("断开过期的运行过程流失败", error))
            return
          }
          connectionId = id
          for (const event of early.splice(0)) deliver(event)
        },
        (error: unknown) => {
          if (closed) return
          release()
          handlers.closed(error)
        },
      )

      return () => {
        if (closed) return
        const established = connectionId
        release()
        if (established === undefined) {
          pending.cancel()
          return
        }
        void bridge.disconnect(established).catch((error: unknown) => console.warn("断开运行过程流失败", error))
      }
    },
  }
}
