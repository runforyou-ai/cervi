/** 应用启动检测状态管理。 */
import { useEffect, useState } from "react"

import { loadStartup, SessionState, type Startup } from "@/api"

type StartupLoadState =
  | { status: "loading"; startup?: never }
  | { status: "loaded"; startup: Startup }

let startupRequest: Promise<Startup> | null = null

/** 首次安装或连接服务器完成后把已缓存的启动检测结果改为就绪，切换路由器后重新挂载的启动检测不再回到入口页。 */
export function markStartupReady() {
  startupRequest = (startupRequest ?? loadStartup()).then((startup) => ({
    ...startup,
    state: SessionState.SessionStateReady,
  }))
}

/** 启动时检测一次当前平台能否进入应用。 */
export function useStartupLoader() {
  const [state, setState] = useState<StartupLoadState>({
    status: "loading",
  })

  useEffect(() => {
    let stale = false
    // 复用当前启动检测请求。
    startupRequest ??= loadStartup().catch((error: unknown) => {
      startupRequest = null
      throw error
    })
    void startupRequest.then(
      (startup) => {
        if (!stale) setState({ status: "loaded", startup })
      },
      (error: unknown) => {
        if (stale) return
        console.warn("启动检测失败，停止加载后续页面", error)
      },
    )
    return () => {
      stale = true
    }
  }, [])

  return state
}
