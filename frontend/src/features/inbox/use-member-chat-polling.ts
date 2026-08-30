/** 控制成员收件箱和当前会话的前台轮询。 */
import { useEffect, useState } from "react"

import { usePortalContainer } from "@/components/ui/portal-container"

export const memberChatPollingInterval = 3_000

/** 判断当前消息页是否同时处于活动标签、可见页面和聚焦窗口。 */
export function useMemberChatPollingActive() {
  const pagePortal = usePortalContainer()
  const [windowActive, setWindowActive] = useState(
    () => document.visibilityState === "visible" && document.hasFocus(),
  )

  useEffect(() => {
    /** 同步浏览器或桌面窗口的前台状态。 */
    function syncWindowState() {
      setWindowActive(
        document.visibilityState === "visible" && document.hasFocus(),
      )
    }

    document.addEventListener("visibilitychange", syncWindowState)
    window.addEventListener("focus", syncWindowState)
    window.addEventListener("blur", syncWindowState)
    return () => {
      document.removeEventListener("visibilitychange", syncWindowState)
      window.removeEventListener("focus", syncWindowState)
      window.removeEventListener("blur", syncWindowState)
    }
  }, [])

  return (pagePortal?.active ?? true) && windowActive
}
