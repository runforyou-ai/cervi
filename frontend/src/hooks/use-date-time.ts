/** 按当前语言格式化日期时间。 */
import { useMemo } from "react"
import { useTranslation } from "react-i18next"

/** 返回按当前语言格式化日期时间的方法。 */
export function useDateTime() {
  const { i18n } = useTranslation()
  const formatter = useMemo(
    () =>
      new Intl.DateTimeFormat(i18n.resolvedLanguage, {
        dateStyle: "medium",
        timeStyle: "short",
      }),
    [i18n.resolvedLanguage]
  )

  return {
    formatDateTime(value: string) {
      return formatter.format(new Date(value))
    },
  }
}
