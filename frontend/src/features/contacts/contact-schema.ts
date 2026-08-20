/** 联系人表单校验规则。 */
import { z } from "zod"
import { isPossiblePhoneNumber } from "react-phone-number-input"

import { ContactStage } from "@/api"
import { requiredWailsEnum } from "@/lib/wails-enum"

/** 创建联系人表单校验。 */
export function createContactSchema(messages: {
  identityRequired: string
  channelRequired: string
  nameTooLong: string
  emailInvalid: string
  phoneInvalid: string
  notesTooLong: string
}) {
  return z
    .object({
      displayName: z.string().trim().max(200, messages.nameTooLong),
      channelId: z.string().uuid(messages.channelRequired),
      stage: requiredWailsEnum(ContactStage),
      email: z.union([z.literal(""), z.string().trim().email(messages.emailInvalid)]),
      phone: z
        .string()
        .refine(
          (value) => value === "" || isPossiblePhoneNumber(value),
          messages.phoneInvalid,
        ),
      notes: z.string().trim().max(5000, messages.notesTooLong),
    })
    .superRefine((value, context) => {
      if (!value.displayName && !value.email && !value.phone) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["displayName"],
          message: messages.identityRequired,
        })
      }
    })
}

export type ContactFormValues = z.infer<ReturnType<typeof createContactSchema>>
