/** 企业信息设置表单校验规则。 */
import { z } from "zod"

/** 创建企业信息设置校验。 */
export function createOrganizationSettingsSchema(messages: {
  nameRequired: string
  nameTooLong: string
}) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .max(32, messages.nameTooLong),
  })
}

export type OrganizationSettingsFormValues = z.infer<
  ReturnType<typeof createOrganizationSettingsSchema>
>
