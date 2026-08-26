/** 用户偏好设置表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useTheme } from "next-themes"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  Locale,
  updateUserPreferences,
  type CurrentUser,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { Switch } from "@/components/ui/switch"
import { NotificationPermissionSettings } from "@/features/notifications/notification-permission-settings"
import {
  AppearanceSettings,
  type ThemePreference,
} from "@/features/settings/appearance-settings"
import {
  createUserPreferencesSchema,
  type UserPreferencesFormValues,
} from "@/features/settings/user-preferences-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { supportedTimeZones } from "@/lib/time-zones"
import {
  readNotificationDevicePreferences,
  setNotificationSoundEnabled,
  type NotificationDeviceScope,
} from "@/platform/notifications"

/** 修改当前用户的界面语言、时区、主题和通知偏好。 */
export function UserPreferencesForm({
  user,
  onUpdated,
}: {
  user: CurrentUser
  onUpdated: (user: CurrentUser) => void
}) {
  const { t, i18n } = useTranslation("settings")
  const navigate = useNavigate()
  const { theme, setTheme } = useTheme()
  const notificationScope = useMemo<NotificationDeviceScope>(
    () => ({ organizationId: user.organizationId, userId: user.id }),
    [user.id, user.organizationId],
  )
  const schema = useMemo(() => createUserPreferencesSchema(t), [t])
  const timeZones = useMemo(
    () => supportedTimeZones(user.timeZone),
    [user.timeZone],
  )
  const form = useForm<UserPreferencesFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      locale: user.locale as UserPreferencesFormValues["locale"],
      timeZone: user.timeZone,
      theme: (theme ?? "system") as ThemePreference,
      messageNotificationsEnabled: user.messageNotificationsEnabled,
      notificationSoundEnabled:
        readNotificationDevicePreferences(notificationScope).soundEnabled,
    },
  })
  /** next-themes 初始化后同步未编辑的主题字段。 */
  useEffect(() => {
    if (!form.formState.dirtyFields.theme) {
      form.setValue("theme", (theme ?? "system") as ThemePreference, {
        shouldDirty: false,
      })
    }
  }, [form, theme])

  /** 保存账号与本机偏好设置。 */
  async function save(values: UserPreferencesFormValues) {
    try {
      const updated = await updateUserPreferences({
        locale: values.locale,
        timeZone: values.timeZone,
        messageNotificationsEnabled: values.messageNotificationsEnabled,
      })
      setTheme(values.theme)
      setNotificationSoundEnabled(
        notificationScope,
        values.notificationSoundEnabled,
      )
      form.reset({
        locale: values.locale,
        timeZone: updated.timeZone,
        theme: values.theme,
        messageNotificationsEnabled: updated.messageNotificationsEnabled,
        notificationSoundEnabled: values.notificationSoundEnabled,
      })
      onUpdated(updated)
      await i18n.changeLanguage(updated.locale)
      console.info("偏好设置已保存", {
        user_id: updated.id,
        locale: updated.locale,
        time_zone: updated.timeZone,
        theme: values.theme,
        message_notifications_enabled: updated.messageNotificationsEnabled,
        notification_sound_enabled: values.notificationSoundEnabled,
      })
      toast.success(t("preferences.saveSuccess"))
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("保存偏好设置失败", error)
      if (isApiError(error)) {
        toast.error(
          apiErrorMessage(error, [
            "locale",
            "timeZone",
            "messageNotificationsEnabled",
          ]),
        )
        return
      }
      toast.error(t("preferences.saveError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form
      className="w-full max-w-2xl space-y-9"
      aria-label={t("preferences.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup className="grid">
        <Controller
          name="theme"
          control={form.control}
          render={({ field, fieldState }) => (
            <AppearanceSettings
              name={field.name}
              value={field.value}
              invalid={fieldState.invalid}
              onBlur={field.onBlur}
              onChange={field.onChange}
            />
          )}
        />
        <Controller
          name="locale"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("preferences.language")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              >
                <option value={Locale.LocaleChineseSimplified}>
                  {t("preferences.languages.zhCN")}
                </option>
                <option value={Locale.LocaleEnglishUnitedStates}>
                  {t("preferences.languages.enUS")}
                </option>
              </NativeSelect>
            </Field>
          )}
        />
        <Controller
          name="timeZone"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("preferences.timeZone")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              >
                {timeZones.map((timeZone) => (
                  <option key={timeZone} value={timeZone}>
                    {timeZone}
                  </option>
                ))}
              </NativeSelect>
            </Field>
          )}
        />
        <section
          className="grid gap-4 border-t pt-5"
          aria-labelledby="notification-preferences-title"
        >
          <h3 id="notification-preferences-title" className="font-medium">
            {t("preferences.notifications.title")}
          </h3>
          <Controller
            name="messageNotificationsEnabled"
            control={form.control}
            render={({ field }) => (
              <Field orientation="horizontal">
                <FieldContent>
                  <FieldLabel htmlFor={field.name}>
                    {t("preferences.notifications.newMessages")}
                  </FieldLabel>
                  <FieldDescription>
                    {t("preferences.notifications.newMessagesDescription")}
                  </FieldDescription>
                </FieldContent>
                <Switch
                  id={field.name}
                  name={field.name}
                  checked={field.value}
                  onBlur={field.onBlur}
                  onCheckedChange={field.onChange}
                  ref={field.ref}
                />
              </Field>
            )}
          />
          <Controller
            name="notificationSoundEnabled"
            control={form.control}
            render={({ field }) => (
              <Field orientation="horizontal">
                <FieldContent>
                  <FieldLabel htmlFor={field.name}>
                    {t("preferences.notifications.sound")}
                  </FieldLabel>
                  <FieldDescription>
                    {t("preferences.notifications.soundDescription")}
                  </FieldDescription>
                </FieldContent>
                <Switch
                  id={field.name}
                  name={field.name}
                  checked={field.value}
                  onBlur={field.onBlur}
                  onCheckedChange={field.onChange}
                  ref={field.ref}
                />
              </Field>
            )}
          />
          <NotificationPermissionSettings />
        </section>
      </FieldGroup>
      <div>
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
          {isSubmitting ? t("preferences.saving") : t("preferences.save")}
        </Button>
      </div>
    </form>
  )
}
