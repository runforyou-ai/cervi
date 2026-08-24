/** 网站渠道对外入口地址和安装代码。 */
import { getServerURL } from "@/api"
import { getAppPlatform } from "@/platform/app-platform"

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
  if (getAppPlatform() === "web") {
    return window.location.origin
  }
  return trimOrigin(await getServerURL())
}

function trimOrigin(origin: string) {
  return origin.replace(/\/+$/, "")
}
