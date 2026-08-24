/** 企业成员和 AI 员工账号状态的表单校验规则。 */
import { z } from "zod"

import { UserStatus } from "@/api"
import { requiredWailsEnum } from "@/lib/wails-enum"

/** 账号状态选项。 */
export const accountStatuses = [
  UserStatus.UserStatusActive,
  UserStatus.UserStatusInactive,
] as const

/** 校验账号状态选择。 */
export const accountStatusSchema = z.object({
  status: requiredWailsEnum(UserStatus),
})

export type AccountStatusFormValues = z.infer<typeof accountStatusSchema>
