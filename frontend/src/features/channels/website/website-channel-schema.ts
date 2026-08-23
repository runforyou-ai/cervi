/** 网站渠道基础信息表单校验规则。 */
import { z } from "zod"

import { ChannelType, Locale } from "@/api"
import {
  createChannelReceptionFields,
  validateChannelReceptionFallback,
} from "@/features/channels/reception/channel-reception-schema"
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
  teamRequired: string
  memberRequired: string
  fallbackDifferent: string
}) {
  return z
    .object({
      type: requiredWailsEnum(ChannelType),
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
      ...createChannelReceptionFields(messages),
    })
    .superRefine((value, context) =>
      validateChannelReceptionFallback(
        value,
        context,
        messages.fallbackDifferent,
      ),
    )
}

export type WebsiteChannelFormValues = z.infer<
  ReturnType<typeof createWebsiteChannelSchema>
>
