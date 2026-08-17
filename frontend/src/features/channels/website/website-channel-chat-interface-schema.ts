import { z } from "zod"

export const defaultWebsiteChannelThemeColor = "#2563EB"

function unicodeLength(value: string) {
  return Array.from(value).length
}

export function isWebsiteChannelThemeColor(
  value: string | undefined
): value is string {
  return /^#[0-9A-Fa-f]{6}$/.test(value ?? "")
}

export function createWebsiteChannelChatInterfaceSchema(messages: {
  titleRequired: string
  titleTooLong: string
  subtitleTooLong: string
  greetingTooLong: string
  themeColorInvalid: string
}) {
  return z.object({
    title: z
      .string()
      .trim()
      .min(1, messages.titleRequired)
      .refine((value) => unicodeLength(value) <= 100, {
        message: messages.titleTooLong,
      }),
    subtitle: z
      .string()
      .trim()
      .refine((value) => unicodeLength(value) <= 120, {
        message: messages.subtitleTooLong,
      }),
    greetingMessage: z
      .string()
      .trim()
      .refine((value) => unicodeLength(value) <= 500, {
        message: messages.greetingTooLong,
      }),
    themeColor: z
      .string()
      .trim()
      .refine(isWebsiteChannelThemeColor, messages.themeColorInvalid),
  })
}

export type WebsiteChannelChatInterfaceFormValues = z.infer<
  ReturnType<typeof createWebsiteChannelChatInterfaceSchema>
>
