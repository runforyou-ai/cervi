/** 角色设置表单校验规则。 */
import { z } from "zod"

import { PermissionCode } from "@/api"

/** 创建角色设置表单校验。 */
export function createRoleSettingsSchema(messages: {
  nameRequired: string
  nameTooLong: string
  descriptionTooLong: string
}) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .max(50, messages.nameTooLong),
    description: z
      .string()
      .trim()
      .max(200, messages.descriptionTooLong),
    permissions: z.array(z.nativeEnum(PermissionCode)),
  })
}

export type RoleSettingsFormValues = z.infer<
  ReturnType<typeof createRoleSettingsSchema>
>
