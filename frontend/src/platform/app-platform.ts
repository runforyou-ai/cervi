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
  chrome?: {
    webview?: {
      postMessage?: unknown
    }
  }
  wails?: {
    invoke?: unknown
  }
}

/** 根据运行环境和开发预览标记识别应用平台。 */
export function resolveAppPlatform(): AppPlatform {
  // 开发窗口通过地址标记加载移动端页面和交互。
  if (import.meta.env.DEV && new URLSearchParams(window.location.search).get("preview") === "mobile") {
    return "mobile"
  }
  const wailsWindow = window as WailsWindow
  const os = wailsWindow._wails?.environment?.OS
  if (os === "ios" || os === "android") {
    return "mobile"
  }
  if (os === "darwin" || os === "windows" || os === "linux") {
    return "desktop"
  }

  const userAgent = navigator.userAgent.toLowerCase()
  const hasWebKitBridge =
    typeof wailsWindow.webkit?.messageHandlers?.external?.postMessage ===
    "function"
  const hasWindowsBridge =
    typeof wailsWindow.chrome?.webview?.postMessage === "function"
  const hasAndroidBridge = typeof wailsWindow.wails?.invoke === "function"

  if (
    hasAndroidBridge ||
    (hasWebKitBridge && /iphone|ipad|ipod/.test(userAgent))
  ) {
    return "mobile"
  }
  if (hasWindowsBridge || hasWebKitBridge) {
    return "desktop"
  }
  return "web"
}

/** 判断桌面端是否运行在 macOS。 */
export function isDesktopMacOS(): boolean {
  if (resolveAppPlatform() !== "desktop") {
    return false
  }
  const wailsWindow = window as WailsWindow
  if (wailsWindow._wails?.environment?.OS === "darwin") {
    return true
  }
  return (
    typeof wailsWindow.webkit?.messageHandlers?.external?.postMessage ===
      "function" && /macintosh|mac os x/i.test(navigator.userAgent)
  )
}
