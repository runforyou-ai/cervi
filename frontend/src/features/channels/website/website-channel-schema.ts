import { z } from "zod"

import { Locale } from "@/api/channels"
import { requiredWailsEnum } from "@/lib/wails-enum"

function unicodeLength(value: string) {
  return Array.from(value).length
}

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
