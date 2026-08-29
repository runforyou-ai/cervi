/** 读取启动入口、登录身份并提供会话状态路由。 */
import { SessionState } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import {
  LoadIdentity,
  LoadStartup,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { bind } from "@/api/client"
import { resolveAppPlatform } from "@/platform/app-platform"

/** 读取初始化或服务器连接入口。 */
export const loadStartup = bind(LoadStartup)

/** 读取当前登录身份。 */
export const loadIdentity = bind(LoadIdentity)

/** 将会话状态映射为路由。 */
export function sessionPath(state: string) {
  switch (state) {
    case SessionState.SessionStateLogin:
      return "/login"
    case SessionState.SessionStateSetup:
      return resolveAppPlatform() === "web" ? "/setup" : "/connect"
    case SessionState.SessionStateConnect:
      return "/connect"
    default:
      return null
  }
}
