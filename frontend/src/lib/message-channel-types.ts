/** 消息渠道类型的界面元数据。 */
import {
  GlobeIcon,
  MessageCircleIcon,
  SendIcon,
  type LucideIcon,
} from "lucide-react"

import { ChannelType } from "@/api"

/** 当前支持展示的消息渠道类型；badgeClassName 用于实色角标，softClassName 用于浅底图标。 */
export const messageChannelTypeDefinitions = [
  {
    type: ChannelType.ChannelTypeWebsite,
    translationKey: "website",
    icon: GlobeIcon,
    badgeClassName: "bg-badge-website",
    softClassName: "bg-badge-website/10 text-badge-website",
  },
  {
    type: ChannelType.ChannelTypeTelegram,
    translationKey: "telegram",
    icon: SendIcon,
    badgeClassName: "bg-badge-telegram",
    softClassName: "bg-badge-telegram/10 text-badge-telegram",
  },
  {
    type: ChannelType.ChannelTypeWeChatOfficialAccount,
    translationKey: "wechatOfficialAccount",
    icon: MessageCircleIcon,
    badgeClassName: "bg-badge-wechat",
    softClassName: "bg-badge-wechat/10 text-badge-wechat",
  },
] as const satisfies readonly {
  type: ChannelType
  translationKey: string
  icon: LucideIcon
  badgeClassName: string
  softClassName: string
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
