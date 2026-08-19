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

export const getServerURL = bind(ServerURL)

export async function login(input: LoginInput) {
  return acceptSession(await call((meta) => Login(meta, input)))
}

export async function logout() {
  try {
    await call((meta) => Logout(meta))
  } finally {
    clearSession()
  }
}

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

export async function install(input: InstallWorkspaceInput) {
  return acceptSession(await call((meta) => InstallWorkspace(meta, input)))
}

export async function connectServer(serverUrl: string) {
  await call((meta) => ConnectServer(meta, serverUrl))
  clearSession()
}
