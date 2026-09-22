/** 用户偏好设置表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
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
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import {
  AppearanceSettings,
  type ThemePreference,
} from "@/features/settings/appearance-settings"
import {
  createUserPreferencesSchema,
  type UserPreferencesFormValues,
} from "@/features/settings/user-preferences-schema"
import { useAutoSave } from "@/hooks/use-auto-save"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { changeAppLanguage } from "@/i18n"
import { apiErrorMessage } from "@/lib/form-errors"
import { supportedTimeZones } from "@/lib/time-zones"

/** 修改当前用户偏好设置，移动端不展示多标签页设置。 */
export function UserPreferencesForm({ user }: { user: CurrentUser }) {
  const { t } = useTranslation(["settings", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const { theme, setTheme } = useTheme()
  const schema = useMemo(() => createUserPreferencesSchema(t), [t])
  const timeZones = useMemo(
    () => supportedTimeZones(user.timeZone),
    [user.timeZone],
  )
  const form = useForm<UserPreferencesFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      locale: user.locale as UserPreferencesFormValues["locale"],
      timeZone: user.timeZone,
      theme: (theme ?? "system") as ThemePreference,
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
  const { markSaved } = useAutoSave({ form, schema, save })

  async function save(values: UserPreferencesFormValues) {
    try {
      const updated = await updateUserPreferences({
        locale: values.locale,
        timeZone: values.timeZone,
        // 账号偏好接口需要完整提交，新消息提醒由通知设置页维护，这里沿用当前值。
        messageNotificationsEnabled: user.messageNotificationsEnabled,
      })
      setTheme(values.theme)
      const next = {
        locale: values.locale,
        timeZone: updated.timeZone,
        theme: values.theme,
      }
      form.reset(next)
      markSaved(next)
      void invalidate(resourceKeys.identity())
      await changeAppLanguage(updated.locale)
      return true
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return false
      }
      console.warn("保存偏好设置失败", error)
      if (isApiError(error)) {
        toast.error(
          apiErrorMessage(error, ["locale", "timeZone"]),
        )
        return false
      }
      toast.error(t("preferences.saveError"))
      return false
    }
  }


  return (
    <form
      className="w-full"
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
      </FieldGroup>
    </form>
  )
}
