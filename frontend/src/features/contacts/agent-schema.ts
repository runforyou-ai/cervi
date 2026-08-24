/** AI 员工表单校验规则。 */
import { z } from "zod"

import { WorkStatus } from "@/api"
import { requiredWailsEnum } from "@/lib/wails-enum"

/** 创建 AI 员工表单校验规则。 */
export function createAgentSchema(messages: { nameRequired: string }) {
  return z.object({
    displayName: z.string().trim().min(1, messages.nameRequired),
    teamIds: z.array(z.string().uuid()),
  })
}

export type AgentFormValues = z.infer<ReturnType<typeof createAgentSchema>>

/** 校验 AI 员工工作状态。 */
export const agentWorkStatusSchema = z.object({
  workStatus: requiredWailsEnum(WorkStatus),
})

export type AgentWorkStatusFormValues = z.infer<
  typeof agentWorkStatusSchema
>
