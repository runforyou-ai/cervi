/** 根据应用服务错误恢复会话入口路由。 */
import type { NavigateFunction } from "react-router"

import { isApiError, sessionPath, SessionState } from "@/api"
import { clearToken } from "@/api/client"

/** 将带有会话状态的错误导航到对应入口。 */
export function recoverSession(
  error: unknown,
  navigate: NavigateFunction,
): boolean {
  if (!isApiError(error)) {
    return false
  }
  const path = sessionPath(error.state)
  if (!path) {
    return false
  }
  if (error.state === SessionState.SessionStateLogin) {
    clearToken()
  }
  navigate(path, { replace: true })
  return true
}
