/** 消息渠道及类型扩展调用与归一化。 */
import {
  ActivateMessageChannel,
  CreateMessageChannel,
  DeactivateMessageChannel,
  GetMessageChannel,
  GetTelegramChannel,
  GetWebsiteChannel,
  ListChannelOptions,
  ListMessageChannels,
  SaveTelegramChannelConnection,
  TestTelegramChannelConnection,
  UpdateMessageChannel,
  UpdateWebsiteChannelAccess,
  UpdateWebsiteChannelChatInterface,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  WebsiteChannel as GeneratedWebsiteChannel,
  WebsiteChannelAccess as GeneratedWebsiteChannelAccess,
  WebsiteChannelAccessInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind, isApiError } from "@/api/client"
import { asList } from "@/api/normalize"

export type WebsiteChannelAccessData = Omit<
  GeneratedWebsiteChannelAccess,
  "allowedHosts"
> & {
  allowedHosts: string[]
}

export type WebsiteChannelData = Omit<GeneratedWebsiteChannel, "access"> & {
  access: WebsiteChannelAccessData
}

const getWebsiteChannelBound = bind(GetWebsiteChannel)
const getTelegramChannelBound = bind(GetTelegramChannel)
const getMessageChannelBound = bind(GetMessageChannel)
const updateWebsiteChannelAccessBound = bind(UpdateWebsiteChannelAccess)
const listChannelOptionsBound = bind(ListChannelOptions)
const listMessageChannelsBound = bind(ListMessageChannels)

/** 读取网站渠道详情。 */
export function getWebsiteChannel(channelId: string, signal?: AbortSignal) {
  return getWebsiteChannelBound(channelId, signal).then(
    (channel): WebsiteChannelData => ({
      // 归一化网站渠道详情。
      ...channel,
      access: normalizeWebsiteChannelAccess(channel.access),
    }),
  )
}

/** 读取消息渠道基础信息。 */
export function getMessageChannel(channelId: string, signal?: AbortSignal) {
  return getMessageChannelBound(channelId, signal)
}

/** 读取 Telegram 渠道详情。 */
export function getTelegramChannel(channelId: string, signal?: AbortSignal) {
  return getTelegramChannelBound(channelId, signal)
}

/** 创建消息渠道。 */
export const createMessageChannel = bind(CreateMessageChannel)

/** 修改消息渠道基础信息。 */
export const updateMessageChannel = bind(UpdateMessageChannel)

/** 测试 Telegram 草稿 Token。 */
export const testTelegramChannelConnection = bind(
  TestTelegramChannelConnection,
)

/** 保存 Telegram 机器人和 Webhook 设置。 */
export const saveTelegramChannelConnection = bind(
  SaveTelegramChannelConnection,
)

/** 判断保存是否需要用户确认复用其他渠道的 Telegram Bot。 */
export function isTelegramBotReuseConfirmationError(error: unknown) {
  return (
    isApiError(error) &&
    error.kind === "conflict" &&
    error.reason === "telegram_bot_reuse_confirmation_required"
  )
}

/** 修改网站渠道聊天界面。 */
export const updateWebsiteChannelChatInterface = bind(
  UpdateWebsiteChannelChatInterface,
)

/** 修改网站渠道允许使用的网站。 */
export function updateWebsiteChannelAccess(
  channelId: string,
  input: WebsiteChannelAccessInput,
) {
  return updateWebsiteChannelAccessBound(channelId, input).then(
    normalizeWebsiteChannelAccess,
  )
}

/** 停用消息渠道。 */
export const deactivateMessageChannel = bind(DeactivateMessageChannel)

/** 启用消息渠道。 */
export const activateMessageChannel = bind(ActivateMessageChannel)

/** 读取当前企业的渠道选择项。 */
export function listChannelOptions(signal?: AbortSignal) {
  return listChannelOptionsBound(signal).then((list) => asList(list.channels))
}

/** 读取消息渠道列表。 */
export function listMessageChannels() {
  return listMessageChannelsBound().then((list) => asList(list.channels))
}

/** 归一化网站渠道允许使用的网站。 */
function normalizeWebsiteChannelAccess(
  access: GeneratedWebsiteChannelAccess,
): WebsiteChannelAccessData {
  return { ...access, allowedHosts: asList(access.allowedHosts) }
}
