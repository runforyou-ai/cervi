/** 运营报表调用。 */
import { GetAIPerformanceReport } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type { AIPerformanceReport } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type AIPerformanceReportData = NonNullArrays<AIPerformanceReport>

/** 读取当前企业指定范围内的 AI 客服表现报表。 */
export const getAIPerformanceReport = bind(GetAIPerformanceReport)
