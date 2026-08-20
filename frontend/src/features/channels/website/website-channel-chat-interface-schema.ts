/** 网站渠道聊天界面表单校验规则。 */
import { z } from "zod"

export const defaultWebsiteChannelThemeColor = "#2563EB"

/** 按 Unicode 字符计算长度。 */
function unicodeLength(value: string) {
  return Array.from(value).length
}

/** 判断主题色是否为六位十六进制颜色。 */
export function isWebsiteChannelThemeColor(
  value: string | undefined
): value is string {
  return /^#[0-9A-Fa-f]{6}$/.test(value ?? "")
}

/** 创建网站渠道聊天界面校验。 */
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
