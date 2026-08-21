/** 企业成员表单校验规则。 */
import { z } from "zod"

import { UserRole } from "@/api"
import { requiredWailsEnum } from "@/lib/wails-enum"

/** 创建企业成员表单校验规则。 */
export function createMemberSchema(
  messages: {
    nameRequired: string
    emailRequired: string
    emailInvalid: string
    passwordRequired: string
    passwordTooShort: string
    passwordTooLong: string
    roleRequired: string
  },
  editing: boolean,
) {
  return z.object({
    displayName: z.string().trim().min(1, messages.nameRequired),
    email: z
      .string()
      .trim()
      .min(1, messages.emailRequired)
      .email(messages.emailInvalid),
    password: editing
      ? z.literal("")
      : z
          .string()
          .min(1, messages.passwordRequired)
          .min(8, messages.passwordTooShort)
          .refine(
            (value) => new TextEncoder().encode(value).length <= 72,
            messages.passwordTooLong,
          ),
    role: requiredWailsEnum(UserRole, messages.roleRequired),
    teamIds: z.array(z.string().uuid()),
  })
}

export type MemberFormValues = z.infer<ReturnType<typeof createMemberSchema>>
