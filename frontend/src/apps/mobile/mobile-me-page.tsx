/** 移动端当前用户基础资料页面。 */
import { useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { logout } from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { UserAvatar } from "@/components/user-avatar"
import { Button } from "@/components/ui/button"

/** 展示当前用户基础资料和退出操作。 */
export function MobileMePage() {
  const { t } = useTranslation("mobile")
  const navigate = useNavigate()
  const { identity } = useMobileWorkspace()
  const [loggingOut, setLoggingOut] = useState(false)

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

  return (
    <section className="flex h-full min-h-0 flex-col">
      <header className="flex h-14 shrink-0 items-center border-b px-4">
        <h1 className="text-lg font-semibold tracking-tight">
          {t("me.title")}
        </h1>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-6">
        <div className="mx-auto max-w-lg">
          <section aria-labelledby="mobile-profile-title">
            <h2
              id="mobile-profile-title"
              className="text-sm font-medium text-muted-foreground"
            >
              {t("me.profile")}
            </h2>
            <div className="mt-3 overflow-hidden rounded-xl border bg-card">
              <div className="flex justify-center px-4 py-6">
                <UserAvatar
                  user={identity.user}
                  className="size-16 rounded-2xl text-xl"
                />
              </div>
              <dl className="divide-y border-t px-4">
                <div className="py-4">
                  <dt className="text-xs text-muted-foreground">
                    {t("me.displayName")}
                  </dt>
                  <dd className="mt-1 break-words text-sm font-medium">
                    {identity.user.displayName}
                  </dd>
                </div>
                <div className="py-4">
                  <dt className="text-xs text-muted-foreground">
                    {t("me.email")}
                  </dt>
                  <dd className="mt-1 break-all text-sm font-medium">
                    {identity.user.email}
                  </dd>
                </div>
              </dl>
            </div>
          </section>

          <div className="mt-8">
            <Button
              className="h-11 w-full"
              variant="destructive"
              disabled={loggingOut}
              onClick={handleLogout}
            >
              {loggingOut ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : null}
              {loggingOut ? t("loggingOut") : t("logout")}
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}
