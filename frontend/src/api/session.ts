/** 读取当前会话入口。 */
import { SessionState } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { LoadSession } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { call, clearToken } from "@/api/client"

/** 读取当前应进入的会话入口，进入登录状态时清除本地令牌。 */
export async function loadSession(signal?: AbortSignal) {
  const session = await call((meta) => LoadSession(meta), signal)
  if (session.state === SessionState.SessionStateLogin) {
    clearToken()
  }
  return session
}
