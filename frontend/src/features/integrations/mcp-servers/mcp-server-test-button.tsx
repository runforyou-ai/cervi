/** 已保存 MCP 服务的行内连接测试。 */
import { useEffect, useRef, useState } from "react"
import { Loader2Icon, PlugZapIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, testSavedMCPServerConnection } from "@/api"
import { Button } from "@/components/ui/button"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 独立维护每一行的连接测试状态。 */
export function MCPServerTestButton({ serverId }: { serverId: string }) {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const [testing, setTesting] = useState(false)
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false }
  }, [])

  /** 测试保存的配置并显示结果。 */
  async function testConnection() {
    if (testing) return
    setTesting(true)
    try {
      await testSavedMCPServerConnection(serverId)
      if (mounted.current) toast.success(t("mcpServer.connection.success"))
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("mcpServer.connection.error"))
    } finally {
      if (mounted.current) setTesting(false)
    }
  }

  return (
    <Button
      variant="outline"
      size="icon-sm"
      aria-label={testing ? t("mcpServer.connection.testing") : t("mcpServer.connection.test")}
      title={testing ? t("mcpServer.connection.testing") : t("mcpServer.connection.test")}
      disabled={testing}
      onClick={() => void testConnection()}
    >
      {testing ? <Loader2Icon className="animate-spin" /> : <PlugZapIcon />}
    </Button>
  )
}
