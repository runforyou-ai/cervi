/** 移动端当前用户与企业连接页面。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon, LogOutIcon, ServerIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { getServerURL, logout } from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { UserAvatar } from "@/components/user-avatar"
import { Button } from "@/components/ui/button"

/** 展示当前身份、企业服务器和会话操作。 */
export function MobileMePage() {
  const { t } = useTranslation("mobile")
  const navigate = useNavigate()
  const { identity } = useMobileWorkspace()
  const [serverURL, setServerURL] = useState("")
  const [serverURLLoaded, setServerURLLoaded] = useState(false)
  const [serverURLFailed, setServerURLFailed] = useState(false)
  const [loggingOut, setLoggingOut] = useState(false)

  useEffect(() => {
    let stale = false
    void getServerURL().then(
      (value) => {
        if (stale) return
        setServerURL(value)
        setServerURLLoaded(true)
      },
      (error: unknown) => {
        if (stale) return
        console.warn("读取企业服务器地址失败", error)
        setServerURLFailed(true)
        setServerURLLoaded(true)
      },
    )
    return () => {
      stale = true
    }
  }, [])

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

  /** 进入共用的企业服务器切换页。 */
  function changeServer() {
    navigate("/connect", { replace: true, state: { from: "me" } })
  }

  return (
    <section className="flex h-full min-h-0 flex-col">
      <header className="flex h-14 shrink-0 items-center border-b px-4">
        <h1 className="text-lg font-semibold tracking-tight">
          {t("me.title")}
        </h1>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-6">
        <div className="mx-auto max-w-lg">
          <div className="flex items-center gap-4">
            <UserAvatar
              user={identity.user}
              className="size-14 rounded-2xl text-lg"
            />
            <div className="min-w-0">
              <p className="truncate text-lg font-semibold">
                {identity.user.displayName}
              </p>
              <p className="mt-0.5 truncate text-sm text-muted-foreground">
                {identity.user.email}
              </p>
            </div>
          </div>

          <dl className="mt-8 divide-y rounded-xl border bg-card px-4">
            <div className="py-4">
              <dt className="text-xs text-muted-foreground">
                {t("organization")}
              </dt>
              <dd className="mt-1 text-sm font-medium">
                {identity.organization.name}
              </dd>
            </div>
            <div className="py-4">
              <dt className="text-xs text-muted-foreground">
                {t("me.server")}
              </dt>
              <dd className="mt-1 break-all text-sm font-medium">
                {!serverURLLoaded
                  ? t("loading")
                  : serverURLFailed
                  ? t("me.serverLoadError")
                  : serverURL || t("me.serverNotSaved")}
              </dd>
            </div>
          </dl>

          <div className="mt-8 grid gap-3">
            <Button
              className="h-11 justify-start"
              variant="outline"
              onClick={changeServer}
            >
              <ServerIcon />
              {t("me.changeServer")}
            </Button>
            <Button
              className="h-11 justify-start"
              variant="destructive"
              disabled={loggingOut}
              onClick={handleLogout}
            >
              {loggingOut ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : (
                <LogOutIcon />
              )}
              {loggingOut ? t("loggingOut") : t("logout")}
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}
