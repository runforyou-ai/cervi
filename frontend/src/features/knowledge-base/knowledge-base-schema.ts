/** 知识库表单校验规则。 */
import { z } from "zod"

/** 知识库名称允许的最大字符数。 */
export const knowledgeBaseNameMaxLength = 120

/** 知识库描述允许的最大字符数。 */
export const knowledgeBaseDescriptionMaxLength = 1000

/** 知识库表单校验规则。 */
export function createKnowledgeBaseSchema(
  messages: {
    nameRequired: string
    nameTooLong: string
    descriptionTooLong: string
  },
) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .max(knowledgeBaseNameMaxLength, messages.nameTooLong),
    description: z
      .string()
      .trim()
      .max(
        knowledgeBaseDescriptionMaxLength,
        messages.descriptionTooLong,
      ),
  })
}

export type KnowledgeBaseFormValues = z.infer<
  ReturnType<typeof createKnowledgeBaseSchema>
>
