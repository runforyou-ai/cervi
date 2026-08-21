/** 按当前用户语言和时区格式化日期时间。 */
import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import { useUserTimeZone } from "@/contexts/user-preferences"

/** 返回按当前用户语言和时区格式化日期时间的方法。 */
export function useDateTime() {
  const { i18n } = useTranslation()
  const timeZone = useUserTimeZone()
  const formatter = useMemo(
    () =>
      new Intl.DateTimeFormat(i18n.resolvedLanguage, {
        timeZone,
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        hourCycle: "h23",
      }),
    [i18n.resolvedLanguage, timeZone],
  )

  return {
    formatDateTime(value: string) {
      const parts = Object.fromEntries(
        formatter
          .formatToParts(new Date(value))
          .filter((part) => part.type !== "literal")
          .map((part) => [part.type, part.value]),
      )
      return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`
    },
  }
}
