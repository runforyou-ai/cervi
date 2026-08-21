/** 偏好表单中的外观主题字段。 */
import { useTranslation } from "react-i18next"

import { Field, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"

/** 定义偏好表单支持的外观主题。 */
export const themePreferences = ["system", "light", "dark"] as const

export type ThemePreference = (typeof themePreferences)[number]

/** 展示与其他偏好字段一致的主题选择框。 */
export function AppearanceSettings({
  name,
  value,
  invalid,
  onBlur,
  onChange,
}: {
  name: string
  value: ThemePreference
  invalid: boolean
  onBlur: () => void
  onChange: (value: ThemePreference) => void
}) {
  const { t } = useTranslation("settings")

  return (
    <Field data-invalid={invalid}>
      <FieldLabel htmlFor={name}>{t("appearance.theme")}</FieldLabel>
      <NativeSelect
        id={name}
        name={name}
        value={value}
        aria-invalid={invalid}
        onBlur={onBlur}
        onChange={(event) =>
          onChange(event.currentTarget.value as ThemePreference)
        }
      >
        {themePreferences.map((preference) => (
          <option key={preference} value={preference}>
            {t(`appearance.options.${preference}`)}
          </option>
        ))}
      </NativeSelect>
    </Field>
  )
}
