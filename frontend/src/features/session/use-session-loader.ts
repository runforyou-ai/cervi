/** 会话加载和暂时不可用识别。 */
import { useCallback, useEffect, useRef, useState } from "react"

import { isUnavailableApiError, loadSession, type Session } from "@/api"

type SessionLoadState = {
  status: "loading" | "loaded" | "unavailable" | "failed"
  session: Session | null
}

/** 加载会话，并识别企业服务器暂时不可用状态。 */
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
      if (isUnavailableApiError(error)) {
        console.warn("企业服务器暂时不可用", error)
        setState({
          status: "unavailable",
          session: null,
        })
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
