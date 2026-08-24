/** 会话加载状态管理。 */
import { useCallback, useEffect, useRef, useState } from "react"

import { loadSession, type Session } from "@/api"

type SessionLoadState = {
  status: "loading" | "loaded" | "failed"
  session: Session | null
}

/** 加载会话，并在无法取得明确会话状态时提供重试入口。 */
export function useSessionLoader() {
  const requestId = useRef(0)
  const [state, setState] = useState<SessionLoadState>({
    status: "loading",
    session: null,
  })

  /** 发起一次会话加载。 */
  const load = useCallback(async () => {
    const currentRequestId = ++requestId.current
    setState((current) => ({ ...current, status: "loading" }))
    try {
      const session = await loadSession()
      if (currentRequestId !== requestId.current) {
        return
      }
      setState({
        status: "loaded",
        session,
      })
    } catch (error) {
      if (currentRequestId !== requestId.current) {
        return
      }
      console.warn("读取会话失败", error)
      setState({
        status: "failed",
        session: null,
      })
    }
  }, [])

  useEffect(() => {
    void load()
    return () => {
      requestId.current += 1
    }
  }, [load])

  return {
    ...state,
    retry: () => void load(),
  }
}
