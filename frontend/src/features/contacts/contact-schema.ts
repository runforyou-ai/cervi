import { z } from "zod"
import { isPossiblePhoneNumber } from "react-phone-number-input"

export function createContactSchema(messages: {
  identityRequired: string
  channelRequired: string
  nameTooLong: string
  emailInvalid: string
  phoneInvalid: string
  notesTooLong: string
}, channelRequired = true) {
  return z
    .object({
      displayName: z.string().trim().max(200, messages.nameTooLong),
      channelId: channelRequired
        ? z.string().uuid(messages.channelRequired)
        : z.union([z.literal(""), z.string().uuid(messages.channelRequired)]),
      stage: z.enum(["visitor", "lead", "customer"]),
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
