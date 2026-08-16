import { useMemo } from "react"
import { useTranslation } from "react-i18next"

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
