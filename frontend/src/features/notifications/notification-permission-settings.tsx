/** 偏好设置中的当前设备通知权限状态与操作。 */
import type { TFunction } from "i18next"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { recoverSession } from "@/lib/session-navigation"
import {
  canSendNotification,
  type NotificationDeviceScope,
} from "@/platform/notifications"
import {
  useNotificationPermission,
  type NotificationPermissionViewState,
} from "@/features/notifications/use-notification-permission"

/** 返回通知权限状态的本地化文案。 */
function permissionStatusLabel(
  status: NotificationPermissionViewState,
  t: TFunction<"settings">,
) {
  switch (status) {
    case "checking":
      return t("preferences.notifications.permission.statuses.checking")
    case "prompt":
      return t("preferences.notifications.permission.statuses.prompt")
    case "granted":
      return t("preferences.notifications.permission.statuses.granted")
    case "denied":
      return t("preferences.notifications.permission.statuses.denied")
    case "system-managed":
      return t("preferences.notifications.permission.statuses.systemManaged")
    case "unsupported":
      return t("preferences.notifications.permission.statuses.unsupported")
  }
}

/** 返回通知权限状态对应的操作说明。 */
function permissionStatusDescription(
  status: NotificationPermissionViewState,
  t: TFunction<"settings">,
) {
  switch (status) {
    case "checking":
      return t("preferences.notifications.permission.descriptions.checking")
    case "prompt":
      return t("preferences.notifications.permission.descriptions.prompt")
    case "granted":
      return t("preferences.notifications.permission.descriptions.granted")
    case "denied":
      return t("preferences.notifications.permission.descriptions.denied")
    case "system-managed":
      return t(
        "preferences.notifications.permission.descriptions.systemManaged",
      )
    case "unsupported":
      return t("preferences.notifications.permission.descriptions.unsupported")
  }
}

/** 展示本设备通知权限，并在需要时提供申请操作。 */
export function NotificationPermissionSettings({
  scope,
}: {
  scope: NotificationDeviceScope
}) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const { status, requesting, requestPermission } =
    useNotificationPermission(scope)

  /** 申请当前设备的通知权限。 */
  async function allowNotifications() {
    try {
      const nextStatus = await requestPermission()
      if (!nextStatus) {
        return
      }
      if (canSendNotification(nextStatus)) {
        toast.success(t("preferences.notifications.permission.allowSuccess"))
        return
      }
      toast.error(t("preferences.notifications.permission.allowDenied"))
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("申请通知权限失败", error)
      toast.error(t("preferences.notifications.permission.allowError"))
    }
  }

  return (
    <div className="grid gap-3 rounded-lg border p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="grid gap-1">
          <p className="text-sm font-medium">
            {t("preferences.notifications.permission.label")}
          </p>
          <p className="text-sm text-muted-foreground">
            {permissionStatusDescription(status, t)}
          </p>
        </div>
        <span className="rounded-full bg-muted px-2 py-1 text-xs font-medium text-muted-foreground">
          {permissionStatusLabel(status, t)}
        </span>
      </div>
      {status === "prompt" ? (
        <div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={requesting}
            onClick={() => void allowNotifications()}
          >
            {requesting ? <LoaderCircleIcon className="animate-spin" /> : null}
            {requesting
              ? t("preferences.notifications.permission.allowing")
              : t("preferences.notifications.permission.allow")}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
