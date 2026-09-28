/** 联系人界面的枚举文案。 */
import type { TFunction } from "i18next"

import type { ChannelType } from "@/api"
import { messageChannelTypeDefinition } from "@/lib/message-channel-types"

/** 渠道类型文案。 */
export function channelTypeLabel(
  type: ChannelType,
  t: TFunction<"contacts">,
) {
  const definition = messageChannelTypeDefinition(type)
  if (!definition) {
    console.warn("未知的渠道类型", type)
    return ""
  }
  return t(`channelTypes.${definition.translationKey}`)
}
