/** AI 员工表单校验规则。 */
import { z } from "zod"

import { AgentExecutionMode, WorkStatus } from "@/api"
import { isAgentModelSelection } from "@/features/contacts/agent-model-selection"
import { requiredWailsEnum } from "@/lib/wails-enum"

const maxSystemInstructionLength = 20000

/** AI 员工表单校验文案。 */
export interface AgentValidationMessages {
  nameRequired: string
  modelRequired: string
  instructionRequired: string
  instructionTooLong: string
}

/** 返回 Unicode 字符数。 */
function unicodeLength(value: string) {
  return [...value].length
}

/** 创建 AI 员工资料校验规则。 */
export function createAgentProfileSchema(messages: { nameRequired: string }) {
  return z.object({
    displayName: z.string().trim().min(1, messages.nameRequired),
    teamIds: z.array(z.string().uuid()),
  })
}

/** 创建 AI 员工平台托管执行配置校验规则。 */
export function createAgentManagedExecutionSchema(
  messages: Omit<AgentValidationMessages, "nameRequired">,
) {
  return z.object({
    modelSelection: z
      .string()
      .min(1, messages.modelRequired)
      .refine(isAgentModelSelection, messages.modelRequired),
    systemInstruction: z
      .string()
      .trim()
      .min(1, messages.instructionRequired)
      .refine(
        (value) => unicodeLength(value) <= maxSystemInstructionLength,
        messages.instructionTooLong,
      ),
  })
}

/** 创建新增 AI 员工表单校验规则。 */
export function createAgentSchema(messages: AgentValidationMessages) {
  return createAgentProfileSchema(messages).extend({
    execution: z.object({
      mode: z.literal(AgentExecutionMode.AgentExecutionModeManaged),
      managed: createAgentManagedExecutionSchema(messages),
    }),
  })
}

export type AgentProfileFormValues = z.infer<
  ReturnType<typeof createAgentProfileSchema>
>

export type AgentManagedExecutionFormValues = z.infer<
  ReturnType<typeof createAgentManagedExecutionSchema>
>

export type AgentFormValues = z.infer<ReturnType<typeof createAgentSchema>>

/** 校验 AI 员工工作状态。 */
export const agentWorkStatusSchema = z.object({
  workStatus: requiredWailsEnum(WorkStatus),
})

export type AgentWorkStatusFormValues = z.infer<
  typeof agentWorkStatusSchema
>
