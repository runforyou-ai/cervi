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

/** 判断首页链接地址是否为 http 或 https 绝对地址。 */
export function isWebsiteHomeLinkURL(value: string) {
  if (value.length > 2048 || !/^https?:\/\//i.test(value)) {
    return false
  }
  try {
    return new URL(value).host !== ""
  } catch {
    return false
  }
}

/** 创建网站渠道聊天界面校验。 */
export function createWebsiteChannelChatInterfaceSchema(messages: {
  titleRequired: string
  titleTooLong: string
  greetingTooLong: string
  themeColorInvalid: string
  homeLinkTitleRequired: string
  homeLinkTitleTooLong: string
  homeLinkURLInvalid: string
}) {
  return z.object({
    title: z
      .string()
      .trim()
      .min(1, messages.titleRequired)
      .refine((value) => unicodeLength(value) <= 100, {
        message: messages.titleTooLong,
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
    homeLinks: z.array(
      z.object({
        title: z
          .string()
          .trim()
          .min(1, messages.homeLinkTitleRequired)
          .refine((value) => unicodeLength(value) <= 100, {
            message: messages.homeLinkTitleTooLong,
          }),
        url: z
          .string()
          .trim()
          .refine(isWebsiteHomeLinkURL, messages.homeLinkURLInvalid),
      })
    ),
  })
}

export type WebsiteChannelChatInterfaceFormValues = z.infer<
  ReturnType<typeof createWebsiteChannelChatInterfaceSchema>
>
