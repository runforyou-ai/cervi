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
import {
  bind,
  clearWebToken,
  invoke,
  requestMeta,
  storeWebToken,
} from "@/api/client"
import { resolveBrowserLanguage } from "@/i18n"
import { beginSessionBoundary } from "@/lib/resource-client"
import { resolveBrowserTimeZone } from "@/lib/time-zones"
import { resolveAppPlatform } from "@/platform/app-platform"

/** 读取已保存的企业服务器地址。 */
export const getServerURL = bind(ServerURL)

/** 登录并建立当前平台会话，保存令牌后进入新的登录会话代次。 */
export async function login(input: LoginInput) {
  const auth = await invoke((meta) => Login(meta, input))
  const identity =
    resolveAppPlatform() === "web" ? storeWebToken(auth) : auth.identity
  beginSessionBoundary()
  return identity
}

/** 退出登录：先清除本地令牌并进入新的登录会话代次，再用原令牌通知企业服务器。 */
export async function logout() {
  const meta = requestMeta()
  clearWebToken()
  beginSessionBoundary()
  await invoke(Logout, meta)
}

/** 初始化企业并保存当前令牌，之后进入新的登录会话代次。 */
export async function install(
  input: Omit<InstallWorkspaceInput, "locale" | "timeZone">,
) {
  const identity = storeWebToken(
    await invoke((meta) =>
      InstallWorkspace(meta, {
        ...input,
        locale: resolveBrowserLanguage() as InstallWorkspaceInput["locale"],
        timeZone: resolveBrowserTimeZone(),
      }),
    ),
  )
  beginSessionBoundary()
  return identity
}

/** 检测企业服务器并返回公开企业名称。 */
export const probeServer = bind(ProbeServer)

/** 进入新的登录会话代次后验证并保存企业服务器地址。 */
export async function connectServer(serverURL: string) {
  beginSessionBoundary()
  await invoke((meta) => ConnectServer(meta, serverURL))
}
