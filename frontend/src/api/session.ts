/** 读取会话入口和对应路由。 */
import { SessionState } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { LoadSession } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { call, clearWebToken } from "@/api/client"

/** 读取当前应进入的会话入口，进入登录状态时清除 Web 端令牌。 */
export async function loadSession(signal?: AbortSignal) {
  const session = await call((meta) => LoadSession(meta), signal)
  if (session.state === SessionState.SessionStateLogin) {
    clearWebToken()
  }
  return session
}

/** 将会话状态映射为路由。 */
export function sessionPath(state: string) {
  switch (state) {
    case SessionState.SessionStateLogin:
      return "/login"
    case SessionState.SessionStateSetup:
      return "/setup"
    case SessionState.SessionStateConnect:
      return "/connect"
    default:
      return null
  }
}
