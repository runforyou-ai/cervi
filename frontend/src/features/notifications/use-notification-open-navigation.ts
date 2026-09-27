/** 原生端点击系统通知后，主界面打开通知对应的会话。 */
import { useEffect } from "react"

import { onNotificationOpened, takeOpenedNotificationPath } from "@/api"
import { navigateToHashPath, workspaceSlugFromHash } from "@/lib/workspace-route"
import { resolveAppPlatform } from "@/platform/app-platform"

/** 读取待打开的通知页面并跳转：挂载时处理点击通知唤起应用的情况，之后在每次通知被点击时处理；只挂在主界面，会话独立窗口不响应。 */
export function useNotificationOpenNavigation() {
  useEffect(() => {
    if (resolveAppPlatform() === "web") {
      return
    }
    let active = true
    /** 取走待打开的页面地址并跳转，没有时不动。 */
    const openPending = () => {
      takeOpenedNotificationPath()
        .then((path) => {
          if (active && path && workspaceSlugFromHash(path)) navigateToHashPath(path)
        })
        .catch((error: unknown) => {
          console.warn("读取通知对应的页面失败", error)
        })
    }
    openPending()
    const unsubscribe = onNotificationOpened(openPending)
    return () => {
      active = false
      unsubscribe()
    }
  }, [])
}
