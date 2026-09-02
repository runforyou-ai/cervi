/** 客户会话回复区校验规则。 */
import { z } from "zod"

/** 创建客户会话文本回复校验规则。 */
export function createConversationComposerSchema(messages: {
  bodyRequired: string
  bodyTooLong: string
}) {
  return z.object({
    body: z
      .string()
      .trim()
      .min(1, messages.bodyRequired)
      .refine(
        (value) => {
          // 按 Unicode 字符计算文本长度。
          return Array.from(value).length <= 4000
        },
        { message: messages.bodyTooLong },
      ),
  })
}

export type ConversationComposerValues = z.infer<
  ReturnType<typeof createConversationComposerSchema>
>
