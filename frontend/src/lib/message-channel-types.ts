/** 消息渠道类型的界面元数据。 */
import {
  GlobeIcon,
  MessageCircleIcon,
  SendIcon,
  type LucideIcon,
} from "lucide-react"

import { ChannelType } from "@/api"

/** 当前支持展示的消息渠道类型。 */
export const messageChannelTypeDefinitions = [
  {
    type: ChannelType.ChannelTypeWebsite,
    translationKey: "website",
    icon: GlobeIcon,
  },
  {
    type: ChannelType.ChannelTypeTelegram,
    translationKey: "telegram",
    icon: SendIcon,
  },
  {
    type: ChannelType.ChannelTypeWeChatOfficialAccount,
    translationKey: "wechatOfficialAccount",
    icon: MessageCircleIcon,
  },
] as const satisfies readonly {
  type: ChannelType
  translationKey: string
  icon: LucideIcon
}[]

type MessageChannelType =
  (typeof messageChannelTypeDefinitions)[number]["type"]

/** 返回消息渠道类型对应的界面元数据。 */
export function messageChannelTypeDefinition(type: ChannelType) {
  return messageChannelTypeDefinitions.find(
    (definition) => definition.type === type,
  )
}

/** 判断路由值是否为当前支持的消息渠道类型。 */
export function isMessageChannelType(
  value: string,
): value is MessageChannelType {
  return messageChannelTypeDefinitions.some(
    (definition) => definition.type === value,
  )
}
