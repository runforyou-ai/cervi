/** 桌面端应用入口和路由。 */
import { useEffect, useState } from "react"
import { Events, Window } from "@wailsio/runtime"

import { SharedAppRoutes } from "@/apps/shared-app-routes"
import { isDesktopMacOS } from "@/platform/app-platform"

/** 判断快捷键是否作用于可编辑文本控件。 */
function isTextEditingTarget(target: EventTarget | null) {
  return (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    (target instanceof HTMLElement && target.isContentEditable)
  )
}

/** 阻止桌面端在非编辑区域执行网页整页全选。 */
function usePreventPageSelectAll() {
  useEffect(() => {
    /** 阻止非编辑区域的全选默认行为。 */
    function preventPageSelectAll(event: KeyboardEvent) {
      if (
        event.key.toLowerCase() === "a" &&
        (event.ctrlKey || event.metaKey) &&
        !event.altKey &&
        !event.shiftKey &&
        !isTextEditingTarget(event.target)
      ) {
        event.preventDefault()
      }
    }

    window.addEventListener("keydown", preventPageSelectAll, true)
    return () =>
      window.removeEventListener("keydown", preventPageSelectAll, true)
  }, [])
}

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
  usePreventPageSelectAll()

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
