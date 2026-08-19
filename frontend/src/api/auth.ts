import {
  LoadSession,
  Login,
  Logout,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type { LoginInput } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { acceptSession, ApiError, call, clearSession } from "@/api/client"

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
