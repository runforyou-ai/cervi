/** 订阅运行中 Agent 的过程流，向调用方提供实时展示状态。 */
import { useEffect, useRef, useState } from "react"
import { useNavigate } from "react-router"

import { createRunStreamClient, type RunStreamState } from "@/api"
import { recoverSession } from "@/lib/session-navigation"

/** 按运行编号订阅过程流；enabled 为 false 时不发起请求，流结束时调用 onEnded 重读持久事实。 */
export function useAgentRunStream(runID: string, enabled: boolean, onEnded: () => Promise<unknown>) {
  const navigate = useNavigate()
  const [state, setState] = useState<RunStreamState>()
  const ended = useRef(onEnded)
  ended.current = onEnded

  useEffect(() => {
    if (!enabled) {
      setState(undefined)
      return
    }
    const client = createRunStreamClient(runID)
    const unsubscribe = client.subscribe((event) => {
      switch (event.type) {
        case "state":
          setState(event.state)
          return
        case "ended":
          // 运行尚未在服务端开始时不重读时间线，避免等待期间反复刷新。
          if (event.delivered) void ended.current().catch(() => undefined)
          return
        case "session_error":
          recoverSession(event.error, navigate)
          return
      }
    })
    client.start()
    return () => {
      unsubscribe()
      client.stop()
    }
  }, [runID, enabled, navigate])

  return state
}
