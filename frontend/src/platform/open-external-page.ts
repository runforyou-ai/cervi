/** 按平台在应用内新窗口或新浏览器标签中打开外部页面。 */
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
