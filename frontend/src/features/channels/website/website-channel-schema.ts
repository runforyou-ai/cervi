/** 网站渠道基础信息表单校验规则。 */
import { z } from "zod"

import { Locale } from "@/api"
import { requiredWailsEnum } from "@/lib/wails-enum"

/** 按 Unicode 字符计算长度。 */
function unicodeLength(value: string) {
  return Array.from(value).length
}

/** 创建网站渠道基础信息校验。 */
export function createWebsiteChannelSchema(messages: {
  nameRequired: string
  nameTooLong: string
  descriptionTooLong: string
}) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .refine((value) => unicodeLength(value) <= 100, {
        message: messages.nameTooLong,
      }),
    description: z
      .string()
      .trim()
      .refine((value) => unicodeLength(value) <= 2000, {
        message: messages.descriptionTooLong,
      }),
    defaultLocale: requiredWailsEnum(Locale),
  })
}

export type WebsiteChannelFormValues = z.infer<
  ReturnType<typeof createWebsiteChannelSchema>
>
