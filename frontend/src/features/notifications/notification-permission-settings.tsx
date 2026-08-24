/** 偏好设置中的当前设备通知权限状态与操作。 */
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldTitle,
} from "@/components/ui/field"
import { recoverSession } from "@/lib/session-navigation"
import {
  canSendNotification,
  openNotificationPermissionSettings,
  type NotificationDeviceScope,
} from "@/platform/notifications"
import { useNotificationPermission } from "@/features/notifications/use-notification-permission"

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
  const authorized = status !== "checking" && canSendNotification(status)

  /** 申请当前设备的通知权限。 */
  async function allowNotifications() {
    try {
      if (
        status === "denied" &&
        (await openNotificationPermissionSettings())
      ) {
        return
      }
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
    <Field orientation="horizontal">
      <FieldContent>
        <FieldTitle>
          {t("preferences.notifications.permission.label")}
        </FieldTitle>
        <FieldDescription>
          {authorized
            ? t("preferences.notifications.permission.authorizedDescription")
            : t(
                "preferences.notifications.permission.unauthorizedDescription",
              )}
        </FieldDescription>
      </FieldContent>
      <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
        {authorized ? (
          <span className="rounded-full bg-muted px-2 py-1 text-xs font-medium text-muted-foreground">
            {t("preferences.notifications.permission.authorized")}
          </span>
        ) : (
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={status === "checking" || requesting}
            onClick={() => void allowNotifications()}
          >
            {requesting ? <LoaderCircleIcon className="animate-spin" /> : null}
            {requesting
              ? t("preferences.notifications.permission.allowing")
              : t("preferences.notifications.permission.allow")}
          </Button>
        )}
      </div>
    </Field>
  )
}
