/** Web 与桌面端工作台布局。 */
import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Outlet, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  loadSession,
  logout,
  sessionPath,
  SessionState,
  type Identity,
} from "@/api"
import { Button } from "@/components/ui/button"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"

/** 读取会话并渲染工作台导航和子页面。 */
export function WorkspaceLayout() {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const [identity, setIdentity] = useState<Identity | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [loggingOut, setLoggingOut] = useState(false)

  /** 读取会话，未就绪则跳转入口。 */
  const fetchIdentity = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const session = await loadSession()
      if (session.state === SessionState.SessionStateReady && session.identity) {
        setIdentity(session.identity)
        console.info("工作台身份已加载", {
          organization: session.identity.organization.name,
        })
        return
      }
      const path = sessionPath(session.state)
      if (path) {
        navigate(path, { replace: true })
        return
      }
      setError(t("loadError"))
    } catch (requestError) {
      console.warn("工作台身份加载失败", requestError)
      setError(t("loadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, t])

  useEffect(() => {
    void fetchIdentity()
  }, [fetchIdentity])

  /** 退出登录并回到登录页。 */
  async function handleLogout() {
    setLoggingOut(true)
    try {
      await logout()
      console.info("用户退出登录")
    } catch (error) {
      console.warn("退出登录失败", error)
      toast.error(t("logoutError"))
    } finally {
      setLoggingOut(false)
      navigate("/login", { replace: true })
    }
  }

  if (loading) {
    return (
      <main className="flex min-h-svh items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </main>
    )
  }

  if (!identity) {
    return (
      <main className="flex min-h-svh items-center justify-center p-6">
        <div className="text-center">
          <p className="text-sm text-muted-foreground">
            {error}
          </p>
          <Button className="mt-4" variant="outline" onClick={fetchIdentity}>
            <RefreshCwIcon />
            {t("retry")}
          </Button>
        </div>
      </main>
    )
  }

  return (
    <div className="flex h-svh min-h-0 w-full overflow-hidden">
      <WorkspaceNavigation
        identity={identity}
        onLogout={handleLogout}
        loggingOut={loggingOut}
      />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
        <Outlet context={identity} />
      </div>
    </div>
  )
}
