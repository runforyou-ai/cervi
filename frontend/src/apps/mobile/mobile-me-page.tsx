/** 移动端当前用户基础资料页面。 */
import { useState } from "react"
import { ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import { logout } from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { UserAvatar } from "@/components/user-avatar"
import { Button } from "@/components/ui/button"
import {
  MobilePageHeader,
  MobilePageState,
  MobileScrollArea,
} from "@/apps/mobile/mobile-page"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"

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
      <MobilePageHeader title={t("me.title")} />
      <MobileScrollArea storageKey="me" className="px-4 py-6">
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
                <div className="py-4">
                  <dt className="text-xs text-muted-foreground">
                    {t("me.organization")}
                  </dt>
                  <dd className="mt-1 break-words text-sm font-medium">
                    {identity.organization.name}
                  </dd>
                </div>
              </dl>
            </div>
          </section>

          <Link
            to="/me/settings"
            state={{ mobileBack: true }}
            className="mt-6 flex min-h-14 items-center justify-between gap-3 border-y text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {t("me.settings")}
            <ChevronRightIcon className="size-4 text-muted-foreground" />
          </Link>
          <div className="mt-9">
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  className="min-h-11 w-full"
                  variant="outline"
                  disabled={loggingOut}
                >
                  {loggingOut ? t("loggingOut") : t("logout")}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("me.logoutTitle")}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("me.logoutDescription")}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel className="min-h-11">
                    {t("cancel")}
                  </AlertDialogCancel>
                  <AlertDialogAction
                    className="min-h-11"
                    onClick={() => void handleLogout()}
                  >
                    {t("logout")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </div>
      </MobileScrollArea>
    </section>
  )
}

/** 保留个人设置的独立页面入口。 */
export function MobileSettingsPage() {
  const { t } = useTranslation("mobile")
  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t("me.settings")} backTo="/me" />
      <MobilePageState
        title={t("unavailable")}
        description={t("me.settingsUnavailable")}
      />
    </section>
  )
}
