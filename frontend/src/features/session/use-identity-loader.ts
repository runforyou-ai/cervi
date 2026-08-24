/** 当前登录身份加载与会话错误分类。 */
import { useEffect, useState } from "react"

import {
  isApiError,
  loadIdentity,
  sessionPath,
  SessionState,
  type Identity,
} from "@/api"
import { clearWebToken } from "@/api/client"

type IdentityLoadState = {
  status: "loading" | "loaded" | "anonymous" | "redirect"
  identity: Identity | null
  redirectPath: string | null
}

/** 读取登录身份，并把明确的会话错误转换成入口。 */
export function useIdentityLoader() {
  const [state, setState] = useState<IdentityLoadState>({
    status: "loading",
    identity: null,
    redirectPath: null,
  })

  useEffect(() => {
    let stale = false
    void loadIdentity().then(
      (identity) => {
        if (!stale) {
          setState({ status: "loaded", identity, redirectPath: null })
        }
      },
      (error: unknown) => {
        if (stale) return
        if (isApiError(error) && error.state === SessionState.SessionStateLogin) {
          console.info("登录状态已失效")
          clearWebToken()
          setState({ status: "anonymous", identity: null, redirectPath: null })
          return
        }
        if (isApiError(error)) {
          const redirectPath = sessionPath(error.state)
          if (redirectPath) {
            console.info("身份接口要求切换入口", { state: error.state })
            setState({ status: "redirect", identity: null, redirectPath })
            return
          }
        }
        console.warn("读取登录身份失败，忽略本次结果", error)
      },
    )
    return () => {
      stale = true
    }
  }, [])

  return state
}
