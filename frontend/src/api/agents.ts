/** 企业 AI 员工调用与归一化。 */
import {
  CreateAgent,
  DeactivateAgent,
  GetAgent,
  ListAgentModelOptions,
  ListAgents,
  ReactivateAgent,
  UpdateAgent,
  UpdateAgentExecution,
  UpdateAgentWorkStatus,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  AgentExecutionMode,
  type Agent,
  type AgentExecutionInput,
  type AgentList,
  type AgentListInput,
  type AgentListItem,
  type AgentWorkStatusInput,
  type CreateAgentInput,
  type UpdateAgentInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type AgentListQuery = Partial<AgentListInput>

export type ManagedAgentExecutionData = Omit<
  Agent["execution"],
  "mode" | "managed"
> & {
  mode: AgentExecutionMode.AgentExecutionModeManaged
  managed: NonNullable<Agent["execution"]["managed"]>
}

export type ManagedAgentExecutionSummaryData = Omit<
  AgentListItem["execution"],
  "mode" | "managed"
> & {
  mode: AgentExecutionMode.AgentExecutionModeManaged
  managed: NonNullable<AgentListItem["execution"]["managed"]>
}

export type AgentData = Omit<Agent, "teams" | "execution"> & {
  teams: NonNullable<Agent["teams"]>
  execution: ManagedAgentExecutionData
}

export type AgentListItemData = Omit<AgentListItem, "teams" | "execution"> & {
  teams: NonNullable<AgentListItem["teams"]>
  execution: ManagedAgentExecutionSummaryData
}

export type AgentListData = Omit<AgentList, "agents"> & {
  agents: AgentListItemData[]
}

const listAgentsBound = bind(ListAgents)
const listAgentModelOptionsBound = bind(ListAgentModelOptions)
const getAgentBound = bind(GetAgent)
const createAgentBound = bind(CreateAgent)
const updateAgentBound = bind(UpdateAgent)
const updateAgentExecutionBound = bind(UpdateAgentExecution)
const updateAgentWorkStatusBound = bind(UpdateAgentWorkStatus)
const deactivateAgentBound = bind(DeactivateAgent)
const reactivateAgentBound = bind(ReactivateAgent)

/** 创建企业 AI 员工。 */
export function createAgent(input: CreateAgentInput) {
  return createAgentBound(input).then(normalizeAgent)
}

/** 读取企业 AI 员工可使用的对话模型。 */
export function listAgentModelOptions() {
  return listAgentModelOptionsBound().then((output) => asList(output.models))
}

/** 读取企业 AI 员工详情。 */
export function getAgent(agentId: string, signal?: AbortSignal) {
  return getAgentBound(agentId, signal).then(normalizeAgent)
}

/** 修改企业 AI 员工。 */
export function updateAgent(agentId: string, input: UpdateAgentInput) {
  return updateAgentBound(agentId, input).then(normalizeAgent)
}

/** 修改企业 AI 员工的执行配置。 */
export function updateAgentExecution(
  agentId: string,
  input: AgentExecutionInput,
) {
  return updateAgentExecutionBound(agentId, input).then(normalizeAgent)
}

/** 修改企业 AI 员工的工作状态。 */
export function updateAgentWorkStatus(
  agentId: string,
  input: AgentWorkStatusInput,
) {
  return updateAgentWorkStatusBound(agentId, input).then(normalizeAgent)
}

/** 禁用企业 AI 员工账号。 */
export function deactivateAgent(agentId: string) {
  return deactivateAgentBound(agentId).then(normalizeAgent)
}

/** 将企业 AI 员工恢复为正常状态。 */
export function reactivateAgent(agentId: string) {
  return reactivateAgentBound(agentId).then(normalizeAgent)
}

/** 读取企业 AI 员工目录。 */
export function listAgents(query: AgentListQuery, signal?: AbortSignal) {
  return listAgentsBound(
    {
      query: query.query ?? "",
      status: query.status ?? null,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output): AgentListData => ({
    ...output,
    agents: asList(output.agents).map((agent): AgentListItemData => {
      // 归一化 AI 员工目录项所属团队和执行配置。
      const execution = agent.execution
      // 归一化 AI 员工平台托管执行配置摘要。
      if (
        execution.mode !== AgentExecutionMode.AgentExecutionModeManaged ||
        execution.managed === undefined ||
        execution.managed === null
      ) {
        throw new Error(`Unsupported agent execution mode: ${execution.mode}`)
      }
      return {
        ...agent,
        teams: asList(agent.teams),
        execution: {
          ...execution,
          mode: execution.mode,
          managed: execution.managed,
        },
      }
    }),
  }))
}

/** 归一化 AI 员工所属团队和执行配置。 */
function normalizeAgent(agent: Agent): AgentData {
  // 归一化 AI 员工平台托管执行配置。
  const execution = agent.execution
  if (
    execution.mode !== AgentExecutionMode.AgentExecutionModeManaged ||
    execution.managed === undefined ||
    execution.managed === null
  ) {
    throw new Error(`Unsupported agent execution mode: ${execution.mode}`)
  }
  return {
    ...agent,
    teams: asList(agent.teams),
    execution: {
      ...execution,
      mode: execution.mode,
      managed: execution.managed,
    },
  }
}
