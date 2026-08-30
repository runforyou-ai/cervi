/** 网站渠道对外入口地址和安装代码。 */
import { resolveChannelServerURL } from "@/features/channels/channel-server-url"

/** 返回网站渠道独立聊天链接。 */
export function websiteChannelChatURL(origin: string, channelId: string) {
  return `${trimOrigin(origin)}/chat/${channelId}`
}

/** 返回网站渠道嵌入安装代码。 */
export function websiteChannelWidgetSnippet(origin: string, channelId: string) {
  return `<script async src="${trimOrigin(origin)}/embed/widget.js?id=${channelId}"></script>`
}

/** 返回网站渠道公开入口使用的源站地址。 */
export async function resolveWebsiteChannelOrigin() {
  return resolveChannelServerURL()
}

/** 移除源站地址末尾多余的斜杠。 */
function trimOrigin(origin: string) {
  return origin.replace(/\/+$/, "")
}
