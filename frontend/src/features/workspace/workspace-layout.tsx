/** Web 与桌面端工作台布局。 */
import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Outlet, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ApiError,
  loadIdentity,
  logout,
  resolveNativeEntry,
  type Identity,
} from "@/api"
import { Button } from "@/components/ui/button"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"

/** 校验登录后显示工作台导航和子页面。 */
export function WorkspaceLayout({
  platform,
}: {
  platform: "web" | "desktop"
}) {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const [identity, setIdentity] = useState<Identity | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [loggingOut, setLoggingOut] = useState(false)

  /** 读取当前登录身份，未登录则跳转。 */
  const fetchIdentity = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      if (platform === "desktop") {
        const entry = await resolveNativeEntry()
        if (entry.status !== "ready") {
          console.info("桌面端入口未就绪", { status: entry.status })
          navigate(entry.status === "connect" ? "/connect" : "/login", {
            replace: true,
          })
          return
        }
        setIdentity(entry.identity)
        console.info("工作台身份已加载", {
          organization: entry.identity.organization.name,
        })
        return
      }
      const currentIdentity = await loadIdentity()
      setIdentity(currentIdentity)
      console.info("工作台身份已加载", {
        organization: currentIdentity.organization.name,
      })
    } catch (requestError) {
      if (requestError instanceof ApiError) {
        if (requestError.code === "INSTALLATION_REQUIRED") {
          console.info("企业未初始化，进入初始化页")
          navigate("/setup", { replace: true })
          return
        }
        if (requestError.code === "AUTH_REQUIRED") {
          console.info("未登录，进入登录页")
          navigate("/login", { replace: true })
          return
        }
      }
      console.warn("工作台身份加载失败", requestError)
      setError(t("loadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, platform, t])

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
