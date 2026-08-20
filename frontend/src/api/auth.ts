/** 登录、登出、企业初始化、初始化状态和企业服务器地址调用。 */
import {
  ConnectServer,
  InstallWorkspace,
  InstallationStatus,
  LoadIdentity,
  Login,
  Logout,
  ServerURL,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  InstallWorkspaceInput,
  LoginInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { ApiError, bind, call, clearToken, storeToken } from "@/api/client"

/** 读取已保存的企业服务器地址。 */
export const getServerURL = bind(ServerURL)

/** 读取企业初始化状态和公开企业名称。 */
export const getInstallationStatus = bind(InstallationStatus)

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

/** 读取当前登录身份；未登录时清除本地令牌。 */
export async function loadIdentity() {
  try {
    return await call((meta) => LoadIdentity(meta))
  } catch (error) {
    if (error instanceof ApiError && error.code === "AUTH_REQUIRED") {
      clearToken()
    }
    throw error
  }
}

/** 初始化企业并保存当前令牌。 */
export async function install(input: InstallWorkspaceInput) {
  return storeToken(await call((meta) => InstallWorkspace(meta, input)))
}

/** 保存企业服务器地址并清除本地令牌。 */
export async function connectServer(serverUrl: string) {
  await call((meta) => ConnectServer(meta, serverUrl))
  clearToken()
}
