/** 提供构建期确定的产品平台。 */

export type AppPlatform = "web" | "desktop" | "mobile"

declare const __CERVI_FRONTEND_TARGET__: AppPlatform

type WailsWindow = Window & {
  _wails?: {
    environment?: {
      OS?: string
    }
  }
}

/** 返回当前构建产物的平台。 */
export function getAppPlatform(): AppPlatform {
  return __CERVI_FRONTEND_TARGET__
}

/** 判断桌面端是否运行在 macOS。 */
export function isDesktopMacOS(): boolean {
  return (window as WailsWindow)._wails?.environment?.OS === "darwin"
}
