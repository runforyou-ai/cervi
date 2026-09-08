/** 移动端个人中心、个人资料和登录与安全入口。 */
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

/** 展示个人资料、登录与安全入口和退出操作。 */
export function MobileMePage() {
  const { t } = useTranslation(["mobile", "common"])
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
      <MobileScrollArea storageKey="me" className="px-4 pt-2 pb-6">
        <div className="mx-auto max-w-lg">
          <Link
            to="/me/profile"
            state={{ mobileBack: true }}
            aria-label={t("me.profile")}
            className="flex items-center gap-3 py-4 text-left outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
          >
            <UserAvatar
              user={identity.user}
              className="size-16 shrink-0 rounded-2xl text-xl"
            />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-base font-semibold">
                {identity.user.displayName}
              </span>
              <span className="mt-1 block truncate text-sm text-muted-foreground">
                {identity.user.email}
              </span>
            </span>
            <ChevronRightIcon
              className="size-5 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
          </Link>
          <Link
            to="/me/security"
            state={{ mobileBack: true }}
            className="flex min-h-14 items-center justify-between gap-3 border-y text-sm outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
          >
            {t("me.security")}
            <ChevronRightIcon
              className="size-5 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
          </Link>
          <div className="mt-9">
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  className="min-h-11 w-full"
                  variant="destructive"
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
                    {t("common:actions.cancel")}
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

/** 展示个人资料占位页并返回个人中心。 */
export function MobileProfilePage() {
  const { t } = useTranslation("mobile")
  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t("me.profile")} backTo="/me" />
      <MobilePageState title={t("unavailable")} />
    </section>
  )
}

/** 展示登录与安全占位页并返回个人中心。 */
export function MobileSecurityPage() {
  const { t } = useTranslation("mobile")
  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t("me.security")} backTo="/me" />
      <MobilePageState title={t("unavailable")} />
    </section>
  )
}
