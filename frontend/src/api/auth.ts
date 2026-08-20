/** 登录、登出、企业初始化和企业服务器地址调用。 */
import {
  ConnectServer,
  InstallWorkspace,
  LoadSession,
  Login,
  Logout,
  ServerURL,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  InstallWorkspaceInput,
  LoginInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { acceptSession, ApiError, bind, call, clearSession } from "@/api/client"

/** 读取已保存的企业服务器地址。 */
export const getServerURL = bind(ServerURL)

/** 登录并保存当前会话。 */
export async function login(input: LoginInput) {
  return acceptSession(await call((meta) => Login(meta, input)))
}

/** 退出登录并清除本地会话。 */
export async function logout() {
  try {
    await call((meta) => Logout(meta))
  } finally {
    clearSession()
  }
}

/** 读取当前登录身份；未登录时清除本地会话。 */
export async function loadSession() {
  try {
    return await call((meta) => LoadSession(meta))
  } catch (error) {
    if (error instanceof ApiError && error.code === "AUTH_REQUIRED") {
      clearSession()
    }
    throw error
  }
}

/** 初始化企业并保存当前会话。 */
export async function install(input: InstallWorkspaceInput) {
  return acceptSession(await call((meta) => InstallWorkspace(meta, input)))
}

/** 保存企业服务器地址并清除本地会话。 */
export async function connectServer(serverUrl: string) {
  await call((meta) => ConnectServer(meta, serverUrl))
  clearSession()
}
