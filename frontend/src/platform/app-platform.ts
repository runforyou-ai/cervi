/** 识别当前运行平台。 */

export type AppPlatform = "web" | "desktop" | "mobile"

type WailsWindow = Window & {
  _wails?: {
    environment?: {
      OS?: string
    }
  }
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: unknown
      }
    }
  }
  wails?: {
    invoke?: unknown
  }
}

/** 根据 Wails 运行环境识别 Web、桌面端和移动端。 */
export function resolveAppPlatform(): AppPlatform {
  const wailsWindow = window as WailsWindow
  const os = wailsWindow._wails?.environment?.OS
  if (os === "ios" || os === "android") {
    return "mobile"
  }
  if (os === "darwin" || os === "windows" || os === "linux") {
    return "desktop"
  }
  const userAgent = navigator.userAgent.toLowerCase()
  const hasIOSBridge =
    typeof wailsWindow.webkit?.messageHandlers?.external?.postMessage ===
    "function"
  const hasAndroidBridge = typeof wailsWindow.wails?.invoke === "function"
  if (
    (hasIOSBridge && /iphone|ipad|ipod/.test(userAgent)) ||
    (hasAndroidBridge && /android/.test(userAgent))
  ) {
    return "mobile"
  }
  return "web"
}

/** 判断桌面端是否运行在 macOS。 */
export function isDesktopMacOS(): boolean {
  return (window as WailsWindow)._wails?.environment?.OS === "darwin"
}
