/** 助理表单校验规则。 */
import { z } from "zod"

import { createAgentManagedExecutionSchema } from "@/features/contacts/agents/agent-schema"
import { displayNamePattern } from "@/lib/display-name"

/** 创建助理资料与执行配置校验规则。 */
export function createAssistantSchema(messages: {
  nameRequired: string
  nameInvalid: string
  modelRequired: string
  instructionTooLong: string
}) {
  return createAgentManagedExecutionSchema(messages).extend({
    displayName: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .regex(displayNamePattern, messages.nameInvalid),
  })
}

export type AssistantFormValues = z.infer<ReturnType<typeof createAssistantSchema>>
