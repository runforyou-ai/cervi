/** 应用启动检测状态管理。 */
import { useEffect, useState } from "react"

import { loadStartup, type Startup } from "@/api"

type StartupLoadState =
  | { status: "loading"; startup?: never }
  | { status: "loaded"; startup: Startup }
  | { status: "failed"; startup?: never }

let startupRequest: Promise<Startup> | null = null

/** 复用当前启动检测请求。 */
function requestStartup() {
  startupRequest ??= loadStartup().catch((error: unknown) => {
    startupRequest = null
    throw error
  })
  return startupRequest
}

/** 启动时检测一次当前平台能否进入应用。 */
export function useStartupLoader() {
  const [revision, setRevision] = useState(0)
  const [state, setState] = useState<StartupLoadState>({
    status: "loading",
  })

  useEffect(() => {
    let stale = false
    void requestStartup().then(
      (startup) => {
        if (!stale) setState({ status: "loaded", startup })
      },
      (error: unknown) => {
        if (stale) return
        console.warn("启动检测失败，停止加载后续页面", error)
        setState({ status: "failed" })
      },
    )
    return () => {
      stale = true
    }
  }, [revision])

  return {
    ...state,
    retry: () => {
      startupRequest = null
      setState({ status: "loading" })
      setRevision((current) => current + 1)
    },
  }
}
