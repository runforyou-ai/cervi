/** 业务系统表单校验规则。 */
import { z } from "zod"

/** 判断地址是否为不含认证信息的完整 HTTP 或 HTTPS 地址。 */
function isBusinessSystemURL(value: string) {
  try {
    const parsed = new URL(value)
    return (
      (parsed.protocol === "http:" || parsed.protocol === "https:") &&
      parsed.host !== "" &&
      parsed.username === "" &&
      parsed.password === ""
    )
  } catch {
    return false
  }
}

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
      .refine(isBusinessSystemURL, messages.urlInvalid),
    enabled: z.boolean(),
  })
}

export type BusinessSystemFormValues = z.infer<
  ReturnType<typeof createBusinessSystemSchema>
>
