/** 识别当前运行平台。 */

export type AppPlatform = "web" | "desktop" | "mobile"
export type DesktopOperatingSystem = "darwin" | "windows" | "linux"

type WailsWindow = Window & {
  _wails?: {
    environment?: {
      OS?: string
    }
  }
}

/** 根据 Wails 运行环境识别 Web、桌面端和移动端。 */
export function resolveAppPlatform(): AppPlatform {
  const os = (window as WailsWindow)._wails?.environment?.OS
  if (os === "ios" || os === "android") {
    return "mobile"
  }
  if (os === "darwin" || os === "windows" || os === "linux") {
    return "desktop"
  }
  return "web"
}

/** 返回当前桌面操作系统，非桌面环境返回空值。 */
export function resolveDesktopOperatingSystem(): DesktopOperatingSystem | null {
  const os = (window as WailsWindow)._wails?.environment?.OS
  if (os === "darwin" || os === "windows" || os === "linux") {
    return os
  }
  return null
}
