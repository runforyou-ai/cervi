/** 运营报表调用。 */
import {
  GetAIPerformanceIssue,
  GetAIPerformanceReport,
  ListAgentServiceSessions,
  ListAIPerformanceBreakdowns,
  ListAIPerformanceIssues,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  AIPerformanceIssue,
  AIPerformanceIssueList,
  AIPerformanceIssueListInput,
  AIPerformanceIssueType,
  AIPerformanceReport,
  ServiceSessionSatisfaction,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { ServiceTranscriptMessageData } from "@/api/knowledge-gaps"
import type { NonNullArrays } from "@/api/normalize"

export type AIPerformanceReportData = NonNullArrays<AIPerformanceReport>

export type AIPerformanceIssueTypeId = Exclude<AIPerformanceIssueType, AIPerformanceIssueType.$zero>

export type AIPerformanceIssueData = Omit<AIPerformanceIssue, "satisfaction"> & {
  satisfaction: Exclude<ServiceSessionSatisfaction, ServiceSessionSatisfaction.$zero> | null
}

export type AIPerformanceIssueListData = Omit<NonNullArrays<AIPerformanceIssueList>, "issues"> & {
  issues: AIPerformanceIssueData[]
}

export type AIPerformanceIssueDetailData = {
  issue: AIPerformanceIssueData
  messages: ServiceTranscriptMessageData[]
}

const listAIPerformanceIssuesBound = bind(ListAIPerformanceIssues)
const getAIPerformanceIssueBound = bind(GetAIPerformanceIssue)

/** 读取当前企业指定范围内的 AI 客服表现概览。 */
export const getAIPerformanceReport = bind(GetAIPerformanceReport)

/** 读取按渠道或咨询分类拆分的一页 AI 客服表现。 */
export const listAIPerformanceBreakdowns = bind(ListAIPerformanceBreakdowns)

/** 读取一页指定类型的问题会话。 */
export function listAIPerformanceIssues(input: AIPerformanceIssueListInput) {
  return listAIPerformanceIssuesBound(input) as Promise<AIPerformanceIssueListData>
}

/** 读取客服周期的质检结论与对客沟通。 */
export function getAIPerformanceIssue(serviceSessionId: string, signal?: AbortSignal) {
  return getAIPerformanceIssueBound(serviceSessionId, signal) as Promise<AIPerformanceIssueDetailData>
}

/** 读取 AI 员工接待的一页服务周期。 */
export const listAgentServiceSessions = bind(ListAgentServiceSessions)
