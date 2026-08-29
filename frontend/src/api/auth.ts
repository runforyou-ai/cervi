/** 登录、登出、企业初始化和企业服务器地址调用。 */
import {
  ConnectServer,
  InstallWorkspace,
  Login,
  Logout,
  ProbeServer,
  ServerURL,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  InstallWorkspaceInput,
  LoginInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind, call, clearWebToken, storeWebToken } from "@/api/client"
import { resolveBrowserLanguage } from "@/i18n"
import { resetResourceCache } from "@/lib/resource-client"
import { resolveBrowserTimeZone } from "@/lib/time-zones"
import { resolveAppPlatform } from "@/platform/app-platform"

/** 读取已保存的企业服务器地址。 */
export const getServerURL = bind(ServerURL)

/** 登录并建立当前平台会话，同时清空上一会话的查询缓存。 */
export async function login(input: LoginInput) {
  const auth = await call((meta) => Login(meta, input))
  resetResourceCache()
  return resolveAppPlatform() === "web" ? storeWebToken(auth) : auth.identity
}

/** 退出登录，清除 Web 端令牌和当前会话的查询缓存。 */
export async function logout() {
  try {
    await call((meta) => Logout(meta))
  } finally {
    clearWebToken()
    resetResourceCache()
  }
}

/** 初始化企业并保存当前令牌，同时清空查询缓存。 */
export async function install(
  input: Omit<InstallWorkspaceInput, "locale" | "timeZone">,
) {
  const identity = storeWebToken(
    await call((meta) =>
      InstallWorkspace(meta, {
        ...input,
        locale: resolveBrowserLanguage() as InstallWorkspaceInput["locale"],
        timeZone: resolveBrowserTimeZone(),
      }),
    ),
  )
  resetResourceCache()
  return identity
}

/** 检测企业服务器并返回公开企业名称，不保存地址。 */
export const probeServer = bind(ProbeServer)

/** 验证并保存企业服务器地址。 */
export const connectServer = bind(ConnectServer)
