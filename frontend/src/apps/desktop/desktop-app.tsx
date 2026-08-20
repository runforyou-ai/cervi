/** 桌面端应用入口和路由。 */
import { useEffect, useState } from "react"
import { Events, Window } from "@wailsio/runtime"

import { SharedAppRoutes } from "@/apps/shared-app-routes"
import { isDesktopMacOS } from "@/platform/app-platform"

/** 同步原生窗口全屏状态。 */
function useWindowFullscreen(enabled: boolean) {
  const [fullscreen, setFullscreen] = useState(false)

  useEffect(() => {
    if (!enabled) {
      return
    }

    void Window.IsFullscreen()
      .then(setFullscreen)
      .catch((error: unknown) => {
        console.warn("读取窗口全屏状态失败", error)
      })

    const stopFullscreen = Events.On(
      Events.Types.Common.WindowFullscreen,
      () => setFullscreen(true),
    )
    const stopUnFullscreen = Events.On(
      Events.Types.Common.WindowUnFullscreen,
      () => setFullscreen(false),
    )

    return () => {
      stopFullscreen()
      stopUnFullscreen()
    }
  }, [enabled])

  return fullscreen
}

/** 渲染桌面端路由。 */
export default function DesktopApp() {
  const macOS = isDesktopMacOS()
  const fullscreen = useWindowFullscreen(macOS)

  return (
    <div
      className="cervi-desktop-app min-h-dvh"
      data-native-os={macOS ? "darwin" : undefined}
      data-window-fullscreen={fullscreen ? "true" : "false"}
    >
      <div aria-hidden="true" className="cervi-window-drag-region" />
      <SharedAppRoutes platform="desktop" />
    </div>
  )
}
