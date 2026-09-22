/** 已保存 MCP 服务的列表行连接测试。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, testSavedMCPServerConnection } from "@/api"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 按服务分别记录连接测试状态，返回正在测试的服务和测试方法。 */
export function useMCPServerConnectionTest() {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const [testingIds, setTestingIds] = useState<ReadonlySet<string>>(new Set())
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 测试保存的配置并显示结果。 */
  async function test(serverId: string) {
    if (testingIds.has(serverId)) return
    setTestingIds((current) => new Set(current).add(serverId))
    try {
      await testSavedMCPServerConnection(serverId)
      if (mounted.current) toast.success(t("mcpServer.connection.success"))
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("mcpServer.connection.error"))
    } finally {
      if (mounted.current)
        setTestingIds((current) => {
          const next = new Set(current)
          next.delete(serverId)
          return next
        })
    }
  }

  return { testingIds, test }
}
