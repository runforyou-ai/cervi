/** 用户通知设置表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  updateUserPreferences,
  type CurrentUser,
} from "@/api"
import { SwitchCardField } from "@/components/form/switch-card-field"
import { FieldGroup } from "@/components/ui/field"
import { NotificationPermissionSettings } from "@/features/notifications/notification-permission-settings"
import {
  createNotificationSettingsSchema,
  type NotificationSettingsFormValues,
} from "@/features/settings/notification-settings-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useAutoSave } from "@/hooks/use-auto-save"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import {
  readNotificationDevicePreferences,
  setNotificationSoundEnabled,
  type NotificationDeviceScope,
} from "@/platform/notifications"

/** 修改新消息提醒、本机通知声音，并管理本设备通知权限。 */
export function NotificationSettingsForm({ user }: { user: CurrentUser }) {
  const { t } = useTranslation(["settings", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const notificationScope = useMemo<NotificationDeviceScope>(
    () => ({ organizationId: user.organizationId, userId: user.id }),
    [user.id, user.organizationId],
  )
  const schema = useMemo(() => createNotificationSettingsSchema(), [])
  const form = useForm<NotificationSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      messageNotificationsEnabled: user.messageNotificationsEnabled,
      notificationSoundEnabled:
        readNotificationDevicePreferences(notificationScope).soundEnabled,
    },
  })
  const markSaved = useAutoSave({ form, schema, save })

  /** 保存新消息提醒到账号偏好，通知声音只写入本机。 */
  async function save(values: NotificationSettingsFormValues) {
    try {
      // 账号偏好接口需要完整提交，语言与时区沿用当前值。
      const updated = await updateUserPreferences({
        locale: user.locale,
        timeZone: user.timeZone,
        messageNotificationsEnabled: values.messageNotificationsEnabled,
      })
      setNotificationSoundEnabled(
        notificationScope,
        values.notificationSoundEnabled,
      )
      const next = {
        messageNotificationsEnabled: updated.messageNotificationsEnabled,
        notificationSoundEnabled: values.notificationSoundEnabled,
      }
      form.reset(next)
      markSaved(next)
      void invalidate(resourceKeys.identity())
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("保存通知设置失败", error)
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["messageNotificationsEnabled"]))
        return
      }
      toast.error(t("notifications.saveError"))
    }
  }

  return (
    <form
      className="w-full"
      aria-label={t("notifications.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup>
        <Controller
          name="messageNotificationsEnabled"
          control={form.control}
          render={({ field }) => (
            <SwitchCardField
              id={field.name}
              name={field.name}
              label={t("notifications.newMessages")}
              description={t("notifications.newMessagesDescription")}
              checked={field.value}
              onBlur={field.onBlur}
              onCheckedChange={field.onChange}
              ref={field.ref}
            />
          )}
        />
        <Controller
          name="notificationSoundEnabled"
          control={form.control}
          render={({ field }) => (
            <SwitchCardField
              id={field.name}
              name={field.name}
              label={t("notifications.sound")}
              description={t("notifications.soundDescription")}
              checked={field.value}
              onBlur={field.onBlur}
              onCheckedChange={field.onChange}
              ref={field.ref}
            />
          )}
        />
        <NotificationPermissionSettings />
      </FieldGroup>
    </form>
  )
}
