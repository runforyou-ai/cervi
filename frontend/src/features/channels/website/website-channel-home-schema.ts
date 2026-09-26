/** 网站渠道聊天窗口首页表单校验规则。 */
import { z } from "zod"

import { WebsiteHomeBlockType } from "@/api"
import { unicodeLength } from "@/features/channels/website/website-channel-chat-interface-schema"

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

/** 创建网站渠道聊天窗口首页校验。 */
export function createWebsiteChannelHomeSchema(messages: {
  welcomeTooLong: string
  headlineTooLong: string
  linkTitleRequired: string
  linkTitleTooLong: string
  linkURLInvalid: string
}) {
  return z.object({
    enabled: z.boolean(),
    welcome: z
      .string()
      .trim()
      .refine((value) => unicodeLength(value) <= 100, {
        message: messages.welcomeTooLong,
      }),
    headline: z
      .string()
      .trim()
      .refine((value) => unicodeLength(value) <= 100, {
        message: messages.headlineTooLong,
      }),
    blocks: z.array(
      z.object({
        type: z.enum([
          WebsiteHomeBlockType.WebsiteHomeBlockRecentConversation,
          WebsiteHomeBlockType.WebsiteHomeBlockStartConversation,
          WebsiteHomeBlockType.WebsiteHomeBlockLinks,
        ]),
        enabled: z.boolean(),
      })
    ),
    links: z.array(
      z.object({
        title: z
          .string()
          .trim()
          .min(1, messages.linkTitleRequired)
          .refine((value) => unicodeLength(value) <= 100, {
            message: messages.linkTitleTooLong,
          }),
        url: z
          .string()
          .trim()
          .refine(isWebsiteHomeLinkURL, messages.linkURLInvalid),
      })
    ),
  })
}

export type WebsiteChannelHomeFormValues = z.infer<
  ReturnType<typeof createWebsiteChannelHomeSchema>
>
