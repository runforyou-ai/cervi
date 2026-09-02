/** 应用启动检测状态管理。 */
import { useEffect, useState } from "react"

import { loadStartup, type Startup } from "@/api"

type StartupLoadState =
  | { status: "loading"; startup?: never }
  | { status: "loaded"; startup: Startup }

let startupRequest: Promise<Startup> | null = null

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
