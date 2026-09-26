/** 登录、官方账号登录、登出、企业初始化和企业服务器地址调用。 */
import {
  CompleteOfficialLogin,
  ConnectServer,
  InstallWorkspace,
  Login,
  Logout,
  ProbeServer,
  ServerURL,
  StartOfficialLogin,
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
import { randomURLSafeString, s256Challenge } from "@/lib/pkce"
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

const officialLoginStoragePrefix = "cervi.officialLogin."

/** 官方账号授权跳转期间按 state 保存的登录尝试与 PKCE verifier。 */
type PendingOfficialLogin = {
  attemptId: string
  codeVerifier: string
}

/** 官方账号登录回调不属于当前浏览器会话发起的登录尝试。 */
export class OfficialLoginStateError extends Error {}

/** 发起官方账号登录，在当前浏览器会话保存 state 对应的登录尝试与 verifier，返回授权地址。 */
export async function startOfficialLogin() {
  const state = randomURLSafeString(24)
  const codeVerifier = randomURLSafeString(48)
  const codeChallenge = await s256Challenge(codeVerifier)
  const started = await invoke((meta) =>
    StartOfficialLogin(meta, { state, nonce: randomURLSafeString(24), codeChallenge }),
  )
  const pending: PendingOfficialLogin = { attemptId: started.attemptId, codeVerifier }
  sessionStorage.setItem(officialLoginStoragePrefix + state, JSON.stringify(pending))
  return started.authorizationUrl
}

/** 用回调中的授权码完成官方账号登录；发起页面仍有效时建立当前平台会话并进入新的登录会话代次，否则返回 null。 */
export async function completeOfficialLogin(state: string, code: string, isCurrent: () => boolean) {
  const key = officialLoginStoragePrefix + state
  const stored = sessionStorage.getItem(key)
  sessionStorage.removeItem(key)
  if (!stored) throw new OfficialLoginStateError()
  const pending = JSON.parse(stored) as PendingOfficialLogin
  const auth = await invoke((meta) =>
    CompleteOfficialLogin(meta, { attemptId: pending.attemptId, code, codeVerifier: pending.codeVerifier }),
  )
  if (!isCurrent()) return null
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
