export type AppPlatform = "web" | "desktop" | "mobile"

type WailsWindow = Window & {
  _wails?: {
    environment?: {
      OS?: string
    }
  }
}

// resolveAppPlatform 根据 Wails 运行环境识别 Web、桌面端和移动端。
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
