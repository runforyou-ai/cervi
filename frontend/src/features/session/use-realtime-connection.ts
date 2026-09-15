/** 登录外壳挂载期间保持成员实时事件流，并把事件流的会话错误恢复到对应入口。 */
import { useEffect } from "react"
import { useNavigate } from "react-router"

import { realtimeClient } from "@/api"
import { recoverSession } from "@/lib/session-navigation"

/** 身份就绪后连接成员实时事件流，卸载时关闭。 */
export function useRealtimeConnection(enabled: boolean) {
  const navigate = useNavigate()

  useEffect(() => {
    if (!enabled) {
      return
    }
    const unsubscribe = realtimeClient.subscribe((event) => {
      if (event.type === "session_error") {
        recoverSession(event.error, navigate)
      }
    })
    // 网络恢复或页面回到前台时跳过剩余退避等待。
    const resume = () => {
      if (document.visibilityState === "visible") {
        realtimeClient.resume()
      }
    }
    window.addEventListener("online", resume)
    document.addEventListener("visibilitychange", resume)
    realtimeClient.start()
    return () => {
      window.removeEventListener("online", resume)
      document.removeEventListener("visibilitychange", resume)
      unsubscribe()
      realtimeClient.stop()
    }
  }, [enabled, navigate])
}
