/** 渠道公开回调和入口复用的企业服务器地址。 */
import { getServerURL } from "@/api"
import { resolveAppPlatform } from "@/platform/app-platform"

/** 返回当前客户端连接的企业服务器地址。 */
export async function resolveChannelServerURL() {
  const serverURL =
    resolveAppPlatform() === "web"
      ? window.location.origin
      : await getServerURL()
  return serverURL.replace(/\/+$/, "")
}
