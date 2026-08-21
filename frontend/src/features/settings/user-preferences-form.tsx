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
  recoverSession,
  updateUserPreferences,
  type User,
} from "@/api"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
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

/** 把用户契约中的语言收窄为表单支持的选项。 */
function formLocale(
  locale: User["locale"],
): UserPreferencesFormValues["locale"] {
  return locale === Locale.LocaleEnglishUnitedStates
    ? Locale.LocaleEnglishUnitedStates
    : Locale.LocaleChineseSimplified
}

/** 把主题设置收窄为表单支持的选项。 */
function formTheme(theme: string | undefined): ThemePreference {
  return theme === "light" || theme === "dark" ? theme : "system"
}

/** 修改当前用户的界面语言、日期时间显示时区和外观主题。 */
export function UserPreferencesForm({
  user,
  onUpdated,
}: {
  user: User
  onUpdated: (user: User) => void
}) {
  const { t, i18n } = useTranslation("settings")
  const navigate = useNavigate()
  const { theme, setTheme } = useTheme()
  const schema = useMemo(() => createUserPreferencesSchema(t), [t])
  const timeZones = useMemo(
    () => supportedTimeZones(user.timeZone),
    [user.timeZone],
  )
  const form = useForm<UserPreferencesFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      locale: formLocale(user.locale),
      timeZone: user.timeZone,
      theme: formTheme(theme),
    },
  })

  useEffect(() => {
    if (!form.formState.dirtyFields.theme) {
      form.setValue("theme", formTheme(theme), { shouldDirty: false })
    }
  }, [form, theme])

  /** 保存全部偏好并立即刷新界面语言、时间和主题。 */
  async function save(values: UserPreferencesFormValues) {
    try {
      const updated = await updateUserPreferences({
        locale: values.locale,
        timeZone: values.timeZone,
      })
      setTheme(values.theme)
      form.reset({
        locale: formLocale(updated.locale),
        timeZone: updated.timeZone,
        theme: values.theme,
      })
      onUpdated(updated)
      await i18n.changeLanguage(updated.locale)
      toast.success(t("preferences.saveSuccess"))
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("保存偏好设置失败", error)
      if (isApiError(error)) {
        let fieldError = false
        for (const name of ["locale", "timeZone"] as const) {
          const message = error.fields[name]
          if (!message) continue
          form.setError(name, { message }, { shouldFocus: !fieldError })
          fieldError = true
        }
        if (fieldError) return
        toast.error(apiErrorMessage(error))
        return
      }
      toast.error(t("preferences.saveError"))
    }
  }

  const { isDirty, isSubmitting } = form.formState

  return (
    <form
      className="mt-6 w-full max-w-xl"
      aria-label={t("preferences.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup className="gap-6">
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
              <FieldError errors={[fieldState.error]} />
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
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />
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
        <div>
          <Button type="submit" disabled={!isDirty || isSubmitting}>
            {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
            {isSubmitting ? t("preferences.saving") : t("preferences.save")}
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
