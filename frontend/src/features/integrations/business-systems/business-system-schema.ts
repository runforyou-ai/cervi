/** 业务系统表单校验规则。 */
import { isHTTPURL } from "@/lib/http-url"
import { z } from "zod"

/** 业务系统描述允许的最大字符数。 */
export const businessSystemDescriptionMaxLength = 200

/** 创建业务系统表单校验。 */
export function createBusinessSystemSchema(messages: {
  nameRequired: string
  nameTooLong: string
  descriptionTooLong: string
  urlRequired: string
  urlTooLong: string
  urlInvalid: string
}) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .max(100, messages.nameTooLong),
    description: z
      .string()
      .trim()
      .max(businessSystemDescriptionMaxLength, messages.descriptionTooLong),
    url: z
      .string()
      .trim()
      .min(1, messages.urlRequired)
      .max(2048, messages.urlTooLong)
      .refine(isHTTPURL, messages.urlInvalid),
    enabled: z.boolean(),
  })
}

export type BusinessSystemFormValues = z.infer<
  ReturnType<typeof createBusinessSystemSchema>
>
