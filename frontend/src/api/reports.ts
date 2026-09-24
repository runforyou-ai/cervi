/** 运营报表调用。 */
import {
  GetAIPerformanceReport,
  ListAIPerformanceBreakdowns,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type { AIPerformanceReport } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type AIPerformanceReportData = NonNullArrays<AIPerformanceReport>

/** 读取当前企业指定范围内的 AI 客服表现概览。 */
export const getAIPerformanceReport = bind(GetAIPerformanceReport)

/** 读取按渠道或咨询分类拆分的一页 AI 客服表现。 */
export const listAIPerformanceBreakdowns = bind(ListAIPerformanceBreakdowns)
