/** 读取启动入口并提供会话状态路由。 */
import { SessionState } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { LoadStartup } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { bind } from "@/api/client"

/** 读取初始化或服务器连接入口。 */
export const loadStartup = bind(LoadStartup)

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
