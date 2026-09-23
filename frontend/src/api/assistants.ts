/** 助理调用。 */
import {
  CreateAssistant,
  DeactivateAssistant,
  GetAssistant,
  ListAssistants,
  ListMemberAssistants,
  MoveAssistant,
  PauseAssistant,
  ReactivateAssistant,
  ResumeAssistant,
  UpdateAssistant,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  AgentExecutionMode,
  AssistantPresence,
  type Assistant,
  type AssistantDetail,
  type AssistantList,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type AssistantPresenceId = Exclude<AssistantPresence, AssistantPresence.$zero>

export type AssistantData = Omit<NonNullArrays<Assistant>, "presence" | "execution"> & {
  presence: AssistantPresenceId
  execution: Omit<NonNullArrays<Assistant>["execution"], "mode" | "managed"> & {
    mode: AgentExecutionMode.AgentExecutionModeManaged
    managed: NonNullable<NonNullArrays<Assistant>["execution"]["managed"]>
  }
}

export type AssistantDetailData = {
  assistant: AssistantData
  execution: Omit<NonNullArrays<AssistantDetail>["execution"], "mode" | "managed"> & {
    mode: AgentExecutionMode.AgentExecutionModeManaged
    managed: NonNullable<NonNullArrays<AssistantDetail>["execution"]["managed"]>
  }
}

export type AssistantListData = { assistants: AssistantData[] }


const listAssistantsBound = bind(ListAssistants)
const listMemberAssistantsBound = bind(ListMemberAssistants)
const getAssistantBound = bind(GetAssistant)
const createAssistantBound = bind(CreateAssistant)
const updateAssistantBound = bind(UpdateAssistant)
const pauseAssistantBound = bind(PauseAssistant)
const resumeAssistantBound = bind(ResumeAssistant)
const moveAssistantBound = bind(MoveAssistant)
const deactivateAssistantBound = bind(DeactivateAssistant)
const reactivateAssistantBound = bind(ReactivateAssistant)

/** 读取当前成员名下的助理。 */
export function listAssistants() {
  return listAssistantsBound().then(asAssistantList)
}

/** 读取指定成员名下的助理。 */
export function listMemberAssistants(userId: string) {
  return listMemberAssistantsBound(userId).then(asAssistantList)
}

/** 读取当前成员名下的助理详情与完整执行配置。 */
export function getAssistant(assistantId: string) {
  return getAssistantBound(assistantId).then(
    (detail) => ({ assistant: asAssistant(detail.assistant), execution: asManagedExecution(detail.execution) }) as AssistantDetailData,
  )
}

/** 在本机创建助理。 */
export function createAssistant(...args: Parameters<typeof createAssistantBound>) {
  return createAssistantBound(...args).then(asAssistant)
}

/** 修改助理的资料与执行配置。 */
export function updateAssistant(...args: Parameters<typeof updateAssistantBound>) {
  return updateAssistantBound(...args).then(asAssistant)
}

/** 暂停助理。 */
export function pauseAssistant(assistantId: string) {
  return pauseAssistantBound(assistantId).then(asAssistant)
}

/** 恢复已暂停的助理。 */
export function resumeAssistant(assistantId: string) {
  return resumeAssistantBound(assistantId).then(asAssistant)
}

/** 把助理换到指定电脑。 */
export function moveAssistant(assistantId: string, deviceId: string) {
  return moveAssistantBound(assistantId, { deviceId }).then(asAssistant)
}

/** 禁用助理。 */
export function deactivateAssistant(assistantId: string) {
  return deactivateAssistantBound(assistantId).then(asAssistant)
}

/** 将助理恢复正常。 */
export function reactivateAssistant(assistantId: string) {
  return reactivateAssistantBound(assistantId).then(asAssistant)
}

/** 断言助理列表中的每一项均为有效在线状态与平台托管执行配置。 */
function asAssistantList(list: NonNullArrays<AssistantList>): AssistantListData {
  return { assistants: list.assistants.map(asAssistant) }
}

/** 断言助理的在线状态有效且使用平台托管执行配置。 */
function asAssistant(assistant: NonNullArrays<Assistant>): AssistantData {
  if (assistant.presence === AssistantPresence.$zero) {
    throw new Error("Assistant presence is missing")
  }
  asManagedExecution(assistant.execution)
  return assistant as AssistantData
}

/** 校验助理使用平台托管执行配置。 */
function asManagedExecution<T extends { mode: AgentExecutionMode; managed?: unknown }>(execution: T) {
  if (execution.mode !== AgentExecutionMode.AgentExecutionModeManaged || !execution.managed) {
    throw new Error(`Unsupported assistant execution mode: ${execution.mode}`)
  }
  return execution
}
