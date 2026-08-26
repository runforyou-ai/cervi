/** 企业通用设置表单校验规则。 */
import { z } from "zod"

/** 创建企业通用设置校验。 */
export function createGeneralSettingsSchema(messages: {
  nameRequired: string
  nameTooLong: string
}) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .max(32, messages.nameTooLong),
    allowArbitraryUrl: z.boolean(),
  })
}

export type GeneralSettingsFormValues = z.infer<
  ReturnType<typeof createGeneralSettingsSchema>
>
