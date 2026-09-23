/** 企业成员表单校验规则。 */
import { z } from "zod"

import { displayNamePattern } from "@/lib/display-name"

/** 创建企业成员表单校验规则。 */
export function createMemberSchema(
  messages: {
    nameRequired: string
    nameInvalid: string
    emailRequired: string
    emailInvalid: string
    passwordRequired: string
    passwordTooShort: string
    passwordTooLong: string
    roleRequired: string
    maxServiceSessionsInvalid: string
  },
  editing: boolean,
) {
  return z.object({
    displayName: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .regex(displayNamePattern, messages.nameInvalid),
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
    roleId: z.string().uuid(messages.roleRequired),
    teamIds: z.array(z.string().uuid()),
    handlesCustomers: z.boolean(),
    maxServiceSessions: z.string().trim(),
  }).superRefine((values, context) => {
    // 最大接待量只在开启接待时校验，必须为正整数。
    if (values.handlesCustomers && !/^[1-9]\d*$/.test(values.maxServiceSessions)) {
      context.addIssue({
        code: "custom",
        path: ["maxServiceSessions"],
        message: messages.maxServiceSessionsInvalid,
      })
    }
  })
}

export type MemberFormValues = z.infer<ReturnType<typeof createMemberSchema>>
