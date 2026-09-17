/** 移动端个人中心、工作状态切换和个人设置子页。 */
import { useState } from "react"
import { CheckIcon, ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import { logout, updateUserWorkStatus, type WorkStatus } from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import {
  MobilePageHeader,
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
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { UserAvatar } from "@/components/user-avatar"
import {
  selectableWorkStatuses,
  WorkStatusDot,
  workStatusLabel,
} from "@/components/work-status"
import { ChangePasswordForm } from "@/features/settings/change-password-form"
import { ProfileSettingsForm } from "@/features/settings/profile-settings-form"
import { UserPreferencesForm } from "@/features/settings/user-preferences-form"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"

const rowClassName =
  "flex min-h-14 w-full items-center gap-3 text-left text-sm outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"

/** 展示个人资料、工作状态、设置入口和退出操作。 */
export function MobileMePage() {
  const { t } = useTranslation(["mobile", "workspace", "common"])
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const { identity } = useMobileWorkspace()
  const [loggingOut, setLoggingOut] = useState(false)
  const [changingWorkStatus, setChangingWorkStatus] = useState(false)

  /** 保存工作状态并刷新当前身份。 */
  async function changeWorkStatus(workStatus: WorkStatus) {
    if (workStatus === identity.user.workStatus) return
    setChangingWorkStatus(true)
    try {
      await updateUserWorkStatus({ workStatus })
      await invalidate(resourceKeys.identity())
    } catch (error) {
      if (!recoverSession(error, navigate)) {
        console.warn("切换工作状态失败", error)
        toast.error(t("workspace:workStatusUpdateError"))
      }
    } finally {
      setChangingWorkStatus(false)
    }
  }

  /** 退出登录并回到登录页。 */
  async function handleLogout() {
    setLoggingOut(true)
    // 先离开工作区，登出清空查询缓存时外壳已经卸载。
    navigate("/login", { replace: true })
    try {
      await logout()
    } catch (error) {
      console.warn("退出登录失败", error)
      toast.error(t("logoutError"))
    } finally {
      setLoggingOut(false)
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
            <span className="relative shrink-0">
              <UserAvatar
                user={identity.user}
                className="size-16 text-xl"
              />
              <WorkStatusDot
                status={identity.user.workStatus}
                className="absolute -right-0.5 -bottom-0.5 size-3.5 ring-2 ring-background"
              />
            </span>
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
          <div className="divide-y border-y">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  className={rowClassName}
                  disabled={changingWorkStatus}
                >
                  <span className="flex-1">{t("workspace:workStatus")}</span>
                  <span className="flex items-center gap-2 text-muted-foreground">
                    <WorkStatusDot status={identity.user.workStatus} />
                    {workStatusLabel(identity.user.workStatus, tCommon)}
                  </span>
                  <ChevronRightIcon
                    className="size-5 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-44">
                {selectableWorkStatuses.map((workStatus) => (
                  <DropdownMenuItem
                    key={workStatus}
                    className="min-h-11"
                    onSelect={() => void changeWorkStatus(workStatus)}
                  >
                    <WorkStatusDot status={workStatus} className="size-2" />
                    <span className="flex-1">
                      {workStatusLabel(workStatus, tCommon)}
                    </span>
                    {identity.user.workStatus === workStatus ? (
                      <CheckIcon className="text-primary" />
                    ) : null}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
            {(["security", "preferences"] as const).map((section) => (
              <Link
                key={section}
                to={`/me/${section}`}
                state={{ mobileBack: true }}
                className={rowClassName}
              >
                <span className="flex-1">{t(`me.${section}`)}</span>
                <ChevronRightIcon
                  className="size-5 shrink-0 text-muted-foreground"
                  aria-hidden="true"
                />
              </Link>
            ))}
          </div>
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

/** 展示个人资料、登录与安全或偏好设置表单，并返回个人中心。 */
export function MobileMeSettingsPage({
  section,
}: {
  section: "profile" | "security" | "preferences"
}) {
  const { t } = useTranslation("mobile")
  const { identity } = useMobileWorkspace()
  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t(`me.${section}`)} backTo="/me" />
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4">
        {section === "profile" ? (
          <ProfileSettingsForm user={identity.user} />
        ) : section === "security" ? (
          <ChangePasswordForm />
        ) : (
          <UserPreferencesForm user={identity.user} />
        )}
      </div>
    </section>
  )
}
