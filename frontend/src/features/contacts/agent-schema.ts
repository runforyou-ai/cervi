/** AI 员工表单校验规则。 */
import { z } from "zod"

/** 创建 AI 员工表单校验规则。 */
export function createAgentSchema(messages: { nameRequired: string }) {
  return z.object({
    displayName: z.string().trim().min(1, messages.nameRequired),
    teamIds: z.array(z.string().uuid()),
  })
}

export type AgentFormValues = z.infer<ReturnType<typeof createAgentSchema>>
