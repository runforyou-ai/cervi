/** 工作区通用设置表单校验规则。 */
import { z } from "zod"

import {
  workspaceNameField,
  workspaceSlugField,
  type WorkspaceFieldMessages,
} from "@/features/account/workspace-schema"

/** 创建工作区通用设置校验。 */
export function createGeneralSettingsSchema(messages: WorkspaceFieldMessages) {
  return z.object({
    name: workspaceNameField(messages),
    slug: workspaceSlugField(messages),
  })
}

export type GeneralSettingsFormValues = z.infer<
  ReturnType<typeof createGeneralSettingsSchema>
>
