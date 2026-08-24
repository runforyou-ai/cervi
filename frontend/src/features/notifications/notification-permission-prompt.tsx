/** 工作台首次通知授权的非阻塞引导。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import { WorkStatus, type CurrentUser } from "@/api"
import { Button } from "@/components/ui/button"
import { useNotificationPermission } from "@/features/notifications/use-notification-permission"
import { recoverSession } from "@/lib/session-navigation"
import {
  canSendNotification,
  dismissNotificationPermissionPrompt,
  isNotificationPermissionPromptHandled,
  subscribeNotificationDevicePreferences,
  type NotificationDeviceScope,
} from "@/platform/notifications"

/** 在满足提醒条件时引导用户主动申请本设备通知权限。 */
export function NotificationPermissionPrompt({ user }: { user: CurrentUser }) {
  const { t } = useTranslation("workspace")
  const location = useLocation()
  const navigate = useNavigate()
  const scope: NotificationDeviceScope = {
    organizationId: user.organizationId,
    userId: user.id,
  }
  const { status, requesting, requestPermission } =
    useNotificationPermission(scope)
  const [handled, setHandled] = useState(() =>
    isNotificationPermissionPromptHandled(scope),
  )

  useEffect(() => {
    const currentScope = {
      organizationId: user.organizationId,
      userId: user.id,
    }

    /** 同步本设备是否已经处理首次授权引导。 */
    function syncHandled() {
      setHandled(isNotificationPermissionPromptHandled(currentScope))
    }

    syncHandled()
    const unsubscribe = subscribeNotificationDevicePreferences(
      currentScope,
      syncHandled,
    )
    window.addEventListener("focus", syncHandled)
    return () => {
      unsubscribe()
      window.removeEventListener("focus", syncHandled)
    }
  }, [user.id, user.organizationId])

  /** 暂不申请权限并在七天后重新显示引导。 */
  function dismissPrompt() {
    dismissNotificationPermissionPrompt(scope)
    setHandled(true)
  }

  /** 从用户点击操作中申请本设备通知权限。 */
  async function allowNotifications() {
    try {
      const nextStatus = await requestPermission()
      if (!nextStatus) {
        return
      }
      setHandled(true)
      if (canSendNotification(nextStatus)) {
        toast.success(t("notificationPermissionPrompt.allowSuccess"))
        return
      }
      toast.error(t("notificationPermissionPrompt.allowDenied"))
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("首次引导申请通知权限失败", error)
      toast.error(t("notificationPermissionPrompt.allowError"))
    }
  }

  if (
    handled ||
    location.pathname === "/account/preferences" ||
    !user.messageNotificationsEnabled ||
    user.workStatus !== WorkStatus.WorkStatusWorking ||
    status !== "prompt"
  ) {
    return null
  }

  return (
    <section
      className="absolute right-4 bottom-4 z-30 grid w-[calc(100%-2rem)] max-w-sm gap-4 rounded-xl border bg-card p-4 text-card-foreground shadow-lg"
      aria-labelledby="notification-permission-prompt-title"
    >
      <div className="grid gap-1.5">
        <h2
          id="notification-permission-prompt-title"
          className="font-semibold"
        >
          {t("notificationPermissionPrompt.title")}
        </h2>
        <p className="text-sm leading-normal text-muted-foreground">
          {t("notificationPermissionPrompt.description")}
        </p>
      </div>
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={requesting}
          onClick={dismissPrompt}
        >
          {t("notificationPermissionPrompt.later")}
        </Button>
        <Button
          type="button"
          size="sm"
          disabled={requesting}
          onClick={() => void allowNotifications()}
        >
          {requesting ? <LoaderCircleIcon className="animate-spin" /> : null}
          {requesting
            ? t("notificationPermissionPrompt.allowing")
            : t("notificationPermissionPrompt.allow")}
        </Button>
      </div>
    </section>
  )
}
