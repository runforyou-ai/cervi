/** 当前登录身份加载与会话错误分类。 */
import { useEffect } from "react"
import { useQuery } from "@tanstack/react-query"

import {
  isApiError,
  loadIdentity,
  sessionPath,
  SessionState,
  type Identity,
} from "@/api"
import { clearWebToken } from "@/api/client"
import { resourceKeys } from "@/hooks/resource-keys"

type IdentityLoadState = {
  status: "loading" | "loaded" | "anonymous" | "redirect" | "failed"
  identity: Identity | null
  redirectPath: string | null
  retry: () => unknown
}

/** 读取登录身份，并把明确的会话错误转换成入口。 */
export function useIdentityLoader(): IdentityLoadState {
  const { data, error, isFetching, refetch: retry } = useQuery({
    queryKey: resourceKeys.identity(),
    queryFn: ({ signal }) => loadIdentity(signal),
  })
  const sessionError = isApiError(error) ? error : null
  const loginExpired = sessionError?.state === SessionState.SessionStateLogin

  const redirectState =
    sessionError && !loginExpired && sessionPath(sessionError.state)
      ? sessionError.state
      : ""
  const failed = Boolean(error) && !loginExpired && !redirectState
  useEffect(() => {
    if (loginExpired) {
      console.info("登录状态已失效")
      clearWebToken()
      return
    }
    if (redirectState) {
      console.info("身份接口要求切换入口", { state: redirectState })
      return
    }
    if (failed) {
      console.warn("读取登录身份失败", error)
    }
  }, [loginExpired, redirectState, failed, error])

  if (loginExpired) {
    return { status: "anonymous", identity: null, redirectPath: null, retry }
  }
  if (redirectState) {
    return {
      status: "redirect",
      identity: null,
      redirectPath: sessionPath(redirectState),
      retry,
    }
  }
  if (data) {
    return { status: "loaded", identity: data, redirectPath: null, retry }
  }
  if (error && !isFetching) {
    return { status: "failed", identity: null, redirectPath: null, retry }
  }
  return { status: "loading", identity: null, redirectPath: null, retry }
}
