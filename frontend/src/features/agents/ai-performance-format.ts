/** AI 表现报表的数字与占比格式、统计天数与待补知识处理状态选项。 */
import { useTranslation } from "react-i18next"

import { KnowledgeGapStatus, type KnowledgeGapStatusId } from "@/api"

/** 可选的统计天数，第一个为默认值。 */
export const periodOptions = [30, 7, 90] as const

/** 待补知识的处理状态筛选，第一个为默认值。 */
export const gapStatuses: KnowledgeGapStatusId[] = [
  KnowledgeGapStatus.KnowledgeGapStatusPending,
  KnowledgeGapStatus.KnowledgeGapStatusAccepted,
  KnowledgeGapStatus.KnowledgeGapStatusDismissed,
]

/** 返回按当前语言格式化计数与占比的方法，分母为 0 时占比显示占位符。 */
export function useAIPerformanceFormat() {
  const { t, i18n } = useTranslation("agents")
  const count = new Intl.NumberFormat(i18n.resolvedLanguage)
  const percent = new Intl.NumberFormat(i18n.resolvedLanguage, {
    style: "percent",
    maximumFractionDigits: 1,
  })
  return {
    /** 格式化计数。 */
    count: (value: number) => count.format(value),
    /** 格式化占比。 */
    rate: (part: number, total: number) =>
      total > 0 ? percent.format(part / total) : t("performance.empty"),
  }
}
