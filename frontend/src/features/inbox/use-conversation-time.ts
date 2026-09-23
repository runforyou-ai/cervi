/** 桌面端与移动端共用的会话时间显示和分钟刷新。 */
import { useCallback, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useUserTimeZone } from "@/contexts/user-preferences"
import { createConversationTimeFormatter } from "./conversation-time"

/** 按当前语言与用户时区格式化最近消息时间。 */
export function useConversationTime() {
  const { t, i18n } = useTranslation("inbox")
  const timeZone = useUserTimeZone()
  return useMemo(
    () => createConversationTimeFormatter(i18n.resolvedLanguage, timeZone, {
      justNow: t("justNow"),
      yesterday: t("yesterday"),
    }),
    [i18n.resolvedLanguage, t, timeZone],
  )
}

/** 把等待起点格式化为等待时长，不足一分钟按一分钟计。 */
export function useWaitingDuration() {
  const { t } = useTranslation("inbox")
  return useCallback((since: string) => {
    const minutes = Math.max(1, Math.floor((Date.now() - new Date(since).getTime()) / 60_000))
    if (minutes < 60) return t("waitingMinutes", { count: minutes })
    if (minutes < 24 * 60) return t("waitingHours", { count: Math.floor(minutes / 60) })
    return t("waitingDays", { count: Math.floor(minutes / (24 * 60)) })
  }, [t])
}

/** 每分钟触发一次重渲染，保持相对时间新鲜。 */
export function useMinuteTick() {
  const [, setTick] = useState(0)
  useEffect(() => {
    const timer = window.setInterval(() => setTick((tick) => tick + 1), 60_000)
    return () => window.clearInterval(timer)
  }, [])
}
