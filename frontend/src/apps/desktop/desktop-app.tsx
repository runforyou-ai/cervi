/** 桌面端应用入口和路由。 */
import { useEffect, useState } from "react"
import { Events, Window } from "@wailsio/runtime"

import { SharedAppRoutes } from "@/apps/shared-app-routes"
import { WindowDragRegion } from "@/components/window-drag-region"
import { resolveDesktopOperatingSystem } from "@/platform/app-platform"

/** 监听原生窗口全屏状态，供 macOS 标题栏布局避让使用。 */
function useWindowFullscreen(enabled: boolean) {
  const [fullscreen, setFullscreen] = useState(false)

  useEffect(() => {
    if (!enabled) {
      return
    }

    let active = true
    void Window.IsFullscreen().then((currentFullscreen) => {
      if (active) {
        setFullscreen(currentFullscreen)
      }
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
      active = false
      stopFullscreen()
      stopUnFullscreen()
    }
  }, [enabled])

  return fullscreen
}

/** 渲染桌面端路由。 */
export default function DesktopApp() {
  const operatingSystem = resolveDesktopOperatingSystem()
  const fullscreen = useWindowFullscreen(operatingSystem === "darwin")

  return (
    <div
      className="cervi-desktop-app min-h-dvh"
      data-native-os={operatingSystem ?? undefined}
      data-window-fullscreen={fullscreen ? "true" : "false"}
    >
      <WindowDragRegion />
      <SharedAppRoutes platform="desktop" />
    </div>
  )
}
