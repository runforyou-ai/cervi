/** 根据应用服务错误恢复全局会话入口。 */
import { useCallback } from "react"

import { isApiError } from "@/api"
import { useSessionController } from "@/features/session/session-context"

/** 返回把结构化接口错误提交给会话控制器的函数。 */
export function useSessionRecovery() {
  const controller = useSessionController()
  return useCallback(
    (error: unknown): boolean => {
      return isApiError(error) && controller.commitClassified(error.state)
    },
    [controller],
  )
}
