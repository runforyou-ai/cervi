/** 联系人界面的枚举文案。 */
import type { TFunction } from "i18next"

import { ChannelType, UserStatus } from "@/api"

/** 渠道类型文案。 */
export function channelTypeLabel(
  type: ChannelType,
  t: TFunction<"contacts">,
) {
  switch (type) {
    case ChannelType.ChannelTypeWebsite:
      return t("channelTypes.website")
    default:
      console.warn("未知的渠道类型", type)
      return ""
  }
}

/** 启用状态文案。 */
export function userStatusLabel(status: UserStatus, t: TFunction<"contacts">) {
  switch (status) {
    case UserStatus.UserStatusActive:
      return t("statuses.active")
    case UserStatus.UserStatusInactive:
      return t("statuses.inactive")
    default:
      console.warn("未知的启用状态", status)
      return ""
  }
}
