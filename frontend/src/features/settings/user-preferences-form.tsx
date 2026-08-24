/** 用户偏好设置表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useTheme } from "next-themes"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  isApiError,
  Locale,
  updateUserPreferences,
  type CurrentUser,
} from "@/api"
import { useSessionRecovery } from "@/lib/session-navigation"
import { Button } from "@/components/ui/button"
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
import { apiErrorMessage } from "@/lib/form-errors"
import { supportedTimeZones } from "@/lib/time-zones"

/** 修改当前用户的界面语言、日期时间显示时区和外观主题。 */
export function UserPreferencesForm({
  user,
  onUpdated,
}: {
  user: CurrentUser
  onUpdated: (user: CurrentUser) => void
}) {
  const { t, i18n } = useTranslation("settings")
  const recoverSession = useSessionRecovery()
  const { theme, setTheme } = useTheme()
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

  /** 保存语言、时区和主题设置。 */
  async function save(values: UserPreferencesFormValues) {
    try {
      const updated = await updateUserPreferences({
        locale: values.locale,
        timeZone: values.timeZone,
      })
      setTheme(values.theme)
      form.reset({
        locale: values.locale,
        timeZone: updated.timeZone,
        theme: values.theme,
      })
      onUpdated(updated)
      await i18n.changeLanguage(updated.locale)
      console.info("偏好设置已保存", {
        user_id: updated.id,
        locale: updated.locale,
        time_zone: updated.timeZone,
        theme: values.theme,
      })
      toast.success(t("preferences.saveSuccess"))
    } catch (error) {
      if (recoverSession(error)) {
        return
      }
      console.warn("保存偏好设置失败", error)
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["locale", "timeZone"]))
        return
      }
      toast.error(t("preferences.saveError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form
      className="w-full max-w-2xl"
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
        <div>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
            {isSubmitting ? t("preferences.saving") : t("preferences.save")}
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
