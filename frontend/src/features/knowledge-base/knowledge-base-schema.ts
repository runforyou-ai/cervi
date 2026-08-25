/** 知识库表单校验规则。 */
import { z } from "zod"

/** 知识库名称允许的最大字符数。 */
export const knowledgeBaseNameMaxLength = 120

/** 知识库描述允许的最大字符数。 */
export const knowledgeBaseDescriptionMaxLength = 1000

/** 外部知识库编号允许的最大字符数。 */
const knowledgeBaseExternalResourceIdMaxLength = 512

/** 知识库表单校验规则。 */
export function createKnowledgeBaseSchema(
  messages: {
    nameRequired: string
    nameTooLong: string
    descriptionTooLong: string
    integrationRequired: string
    externalResourceRequired: string
    externalResourceTooLong: string
  },
  isExternal: boolean,
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
    integrationConnectionId: z
      .string()
      .trim()
      .min(isExternal ? 1 : 0, messages.integrationRequired),
    externalResourceId: z
      .string()
      .trim()
      .min(isExternal ? 1 : 0, messages.externalResourceRequired)
      .max(
        knowledgeBaseExternalResourceIdMaxLength,
        messages.externalResourceTooLong,
      ),
  })
}

export type KnowledgeBaseFormValues = z.infer<
  ReturnType<typeof createKnowledgeBaseSchema>
>
