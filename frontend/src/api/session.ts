/** 读取会话入口，并把会话错误转到对应路由。 */
import type { NavigateFunction } from "react-router"

import {
  ErrorKind,
  SessionState,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { LoadSession } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { call, clearToken, isApiError } from "@/api/client"

/** 读取当前应进入的会话入口。 */
export function loadSession(signal?: AbortSignal) {
  return call((meta) => LoadSession(meta), signal)
}

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

/** 会话未就绪时跳转到对应入口。 */
export function recoverSession(
  error: unknown,
  navigate: NavigateFunction,
): boolean {
  if (!isApiError(error)) {
    return false
  }
  const path =
    sessionPath(error.state) ??
    (error.kind === ErrorKind.ErrorKindUnavailable ? "/connect" : null)
  if (!path) {
    return false
  }
  if (error.state === SessionState.SessionStateLogin) {
    clearToken()
  }
  navigate(path, { replace: true })
  return true
}
