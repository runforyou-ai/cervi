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
import { bind, call, clearToken, storeToken } from "@/api/client"
import { resolveBrowserLanguage } from "@/i18n"
import { resolveBrowserTimeZone } from "@/lib/time-zones"

/** 读取已保存的企业服务器地址。 */
export const getServerURL = bind(ServerURL)

/** 登录并保存当前令牌。 */
export async function login(input: LoginInput) {
  return storeToken(await call((meta) => Login(meta, input)))
}

/** 退出登录并清除本地令牌。 */
export async function logout() {
  try {
    await call((meta) => Logout(meta))
  } finally {
    clearToken()
  }
}

/** 初始化企业并保存当前令牌。 */
export async function install(
  input: Omit<InstallWorkspaceInput, "locale" | "timeZone">,
) {
  return storeToken(
    await call((meta) =>
      InstallWorkspace(meta, {
        ...input,
        locale: resolveBrowserLanguage() as InstallWorkspaceInput["locale"],
        timeZone: resolveBrowserTimeZone(),
      }),
    ),
  )
}

/** 检测企业服务器并返回公开企业名称，不保存地址。 */
export const probeServer = bind(ProbeServer)

/** 保存企业服务器地址并清除本地令牌。 */
export async function connectServer(serverUrl: string) {
  await call((meta) => ConnectServer(meta, serverUrl))
  clearToken()
}
