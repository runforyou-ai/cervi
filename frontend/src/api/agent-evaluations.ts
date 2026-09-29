/** AI 员工回答质量评测调用。 */
import {
  CreateAgentEvaluationCase,
  DeleteAgentEvaluationCase,
  GetAgentEvaluation,
  GetAgentEvaluationCase,
  RerunAgentEvaluationCase,
  StartAgentEvaluationRun,
  UpdateAgentEvaluationCase,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  AgentEvaluationErrorCode,
  AgentEvaluationResultStatus,
  AgentHandoffReason,
  AgentEvaluationRunStatus,
  AgentRunOutcome,
  ServiceAudience,
  type AgentEvaluation,
  type AgentEvaluationAttempt,
  type AgentEvaluationCase,
  type AgentEvaluationCaseDetail,
  type AgentEvaluationCaseInput,
  type AgentEvaluationCaseRow,
  type AgentEvaluationRunSummary,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type AgentEvaluationResultStatusId = Exclude<AgentEvaluationResultStatus, AgentEvaluationResultStatus.$zero>

export type AgentEvaluationActionId = Exclude<AgentRunOutcome, AgentRunOutcome.$zero>

/** 评测用例可选的提问人服务对象。 */
export type AgentEvaluationAudienceId = ServiceAudience.ServiceAudienceCustomer | ServiceAudience.ServiceAudienceEmployee

export type AgentEvaluationErrorCodeId = Exclude<AgentEvaluationErrorCode, AgentEvaluationErrorCode.$zero>

export type AgentEvaluationAttemptData = Omit<NonNullArrays<AgentEvaluationAttempt>, "snapshot" | "status" | "actualAction" | "actualReason" | "errorCode"> & {
  snapshot: Omit<AgentEvaluationAttempt["snapshot"], "audience" | "expectedAction"> & {
    audience: AgentEvaluationAudienceId
    expectedAction: AgentEvaluationActionId
  }
  status: AgentEvaluationResultStatusId
  actualAction: AgentEvaluationActionId | null
  actualReason: Exclude<AgentHandoffReason, AgentHandoffReason.$zero> | null
  errorCode: AgentEvaluationErrorCodeId | null
}

export type AgentEvaluationCaseData = Omit<NonNullArrays<AgentEvaluationCase>, "audience" | "expectedAction"> & {
  audience: AgentEvaluationAudienceId
  expectedAction: AgentEvaluationActionId
}

export type AgentEvaluationCaseRowData = Omit<NonNullArrays<AgentEvaluationCaseRow>, "audience" | "expectedAction" | "latestStatus" | "rerunStatus"> & {
  audience: AgentEvaluationAudienceId
  expectedAction: AgentEvaluationActionId
  latestStatus: AgentEvaluationResultStatusId | null
  rerunStatus: AgentEvaluationResultStatusId | null
}

export type AgentEvaluationRunSummaryData = Omit<AgentEvaluationRunSummary, "status"> & {
  status: Exclude<AgentEvaluationRunStatus, AgentEvaluationRunStatus.$zero>
}

export type AgentEvaluationData = Omit<NonNullArrays<AgentEvaluation>, "latest" | "previous" | "cases"> & {
  latest: AgentEvaluationRunSummaryData | null
  previous: AgentEvaluationRunSummaryData | null
  cases: AgentEvaluationCaseRowData[]
}

export type AgentEvaluationCaseDetailData = Omit<NonNullArrays<AgentEvaluationCaseDetail>, "case" | "attempts"> & {
  case: AgentEvaluationCaseData
  attempts: AgentEvaluationAttemptData[]
}

const getAgentEvaluationBound = bind(GetAgentEvaluation)
const getAgentEvaluationCaseBound = bind(GetAgentEvaluationCase)
const createAgentEvaluationCaseBound = bind(CreateAgentEvaluationCase)
const updateAgentEvaluationCaseBound = bind(UpdateAgentEvaluationCase)

/** 读取 AI 员工评测页的最近两次运行与全部用例。 */
export function getAgentEvaluation(agentId: string, signal?: AbortSignal) {
  return getAgentEvaluationBound(agentId, signal) as Promise<AgentEvaluationData>
}

/** 读取评测用例与它在最近一次运行中的全部尝试。 */
export function getAgentEvaluationCase(agentId: string, caseId: string, signal?: AbortSignal) {
  return getAgentEvaluationCaseBound(agentId, caseId, signal) as Promise<AgentEvaluationCaseDetailData>
}

/** 新建手动评测用例。 */
export function createAgentEvaluationCase(agentId: string, input: AgentEvaluationCaseInput) {
  return createAgentEvaluationCaseBound(agentId, input) as Promise<AgentEvaluationCaseData>
}

/** 修改评测用例。 */
export function updateAgentEvaluationCase(agentId: string, caseId: string, input: AgentEvaluationCaseInput) {
  return updateAgentEvaluationCaseBound(agentId, caseId, input) as Promise<AgentEvaluationCaseData>
}

/** 删除评测用例。 */
export const deleteAgentEvaluationCase = bind(DeleteAgentEvaluationCase)

/** 用 AI 员工当前生效的配置对全部用例发起一次评测运行。 */
export const startAgentEvaluationRun = bind(StartAgentEvaluationRun)

/** 在最近一次运行中重新运行一条用例。 */
export const rerunAgentEvaluationCase = bind(RerunAgentEvaluationCase)
