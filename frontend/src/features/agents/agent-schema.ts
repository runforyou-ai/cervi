/** AI 员工表单校验规则。 */
import { z } from "zod"

import { AgentExecutionMode, WorkStatus } from "@/api"
import { isAgentModelSelection } from "@/features/agents/agent-model-selection"
import { displayNamePattern } from "@/lib/display-name"
import { requiredWailsEnum } from "@/lib/wails-enum"

const maxSystemInstructionLength = 20000

/** AI 员工表单校验文案。 */
export interface AgentValidationMessages {
  nameRequired: string
  nameInvalid: string
  roleRequired: string
  modelRequired: string
  instructionTooLong: string
}

/** 创建 AI 员工资料校验规则。 */
export function createAgentProfileSchema(messages: {
  nameRequired: string
  nameInvalid: string
  roleRequired: string
}) {
  return z.object({
    displayName: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .regex(displayNamePattern, messages.nameInvalid),
    roleId: z.string().uuid(messages.roleRequired),
    workStatus: requiredWailsEnum(WorkStatus),
    teamIds: z.array(z.string().uuid()),
    handlesCustomers: z.boolean(),
  })
}

/** 创建 AI 员工平台托管执行配置校验规则。 */
export function createAgentManagedExecutionSchema(
  messages: Omit<AgentValidationMessages, "nameRequired" | "nameInvalid" | "roleRequired">,
) {
  return z.object({
    modelSelection: z
      .string()
      .min(1, messages.modelRequired)
      .refine(isAgentModelSelection, messages.modelRequired),
    knowledgeBaseIds: z.array(z.string().uuid()),
    systemInstruction: z
      .string()
      .trim()
      .refine((value) => {
        // 校验系统指令的 Unicode 字符数上限。
        return [...value].length <= maxSystemInstructionLength
      }, messages.instructionTooLong),
  })
}

/** 创建新增 AI 员工表单校验规则。 */
export function createAgentSchema(messages: AgentValidationMessages) {
  return createAgentProfileSchema(messages)
    .omit({ workStatus: true })
    .extend({
      execution: z.object({
        mode: z.literal(AgentExecutionMode.AgentExecutionModeManaged),
        managed: createAgentManagedExecutionSchema(messages),
      }),
    })
}

export type AgentProfileFormValues = z.infer<
  ReturnType<typeof createAgentProfileSchema>
>

/** 创建运行配置编辑表单校验规则，服务绑定仅在编辑页配置。 */
export function createAgentExecutionSchema(
  messages: Omit<AgentValidationMessages, "nameRequired" | "nameInvalid" | "roleRequired">,
) {
  return createAgentManagedExecutionSchema(messages).extend({ mcpServerIds: z.array(z.string().uuid()) })
}

export type AgentExecutionFormValues = z.infer<ReturnType<typeof createAgentExecutionSchema>>

export type AgentFormValues = z.infer<ReturnType<typeof createAgentSchema>>
