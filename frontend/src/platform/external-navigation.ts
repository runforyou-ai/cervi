/** 按平台处理应用内页面和系统浏览器外部导航。 */
import { openExternalPage as openExternalPageWindow } from "@/api"
import { resolveAppPlatform } from "@/platform/app-platform"

/** Web 端新开浏览器标签，桌面端在应用内新窗口打开外部页面。 */
export async function openExternalPage(input: { title: string; url: string }) {
  if (resolveAppPlatform() === "desktop") {
    await openExternalPageWindow(input)
    return
  }
  window.open(input.url, "_blank", "noopener,noreferrer")
}

/** 按平台在系统浏览器中打开外部 URL。 */
export async function openExternalURL(url: string) {
  if (resolveAppPlatform() === "web") {
    window.open(url, "_blank", "noopener,noreferrer")
    return
  }

  const { Browser } = await import("@wailsio/runtime")
  await Browser.OpenURL(url)
}
