/** 登录外壳挂载期间保持成员实时事件流与同步协调器，并把会话错误恢复到对应入口。 */
import { useEffect } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"

import { getSyncHeads, realtimeClient } from "@/api"
import { ResourceRefresher } from "@/features/session/resource-refresher"
import { SyncCoordinator } from "@/features/session/sync-coordinator"
import { recoverSession } from "@/lib/session-navigation"

/** 身份就绪后连接成员实时事件流并启动兜底校验，卸载时关闭。 */
export function useRealtimeConnection(enabled: boolean) {
  const navigate = useNavigate()
  const client = useQueryClient()

  useEffect(() => {
    if (!enabled) {
      return
    }
    const refresher = new ResourceRefresher(client)
    const coordinator = new SyncCoordinator({
      invalidate: refresher.invalidate,
      retry: refresher.retry,
      readHeads: () => getSyncHeads(),
      failed: (error) => {
        if (!recoverSession(error, navigate)) {
          console.warn("读取同步探针失败", error)
        }
      },
    })
    const unsubscribe = realtimeClient.subscribe((event) => {
      if (event.type === "frame") {
        coordinator.receive(event.frame)
      } else if (event.type === "session_error") {
        recoverSession(event.error, navigate)
      }
    })
    // 网络恢复或页面回到前台时跳过剩余退避等待，并立即做一次兜底校验。
    const resume = () => {
      if (document.visibilityState === "visible") {
        realtimeClient.resume()
        void coordinator.probe()
      }
    }
    window.addEventListener("online", resume)
    document.addEventListener("visibilitychange", resume)
    realtimeClient.start()
    coordinator.start()
    return () => {
      window.removeEventListener("online", resume)
      document.removeEventListener("visibilitychange", resume)
      unsubscribe()
      realtimeClient.stop()
      coordinator.dispose()
    }
  }, [client, enabled, navigate])
}
