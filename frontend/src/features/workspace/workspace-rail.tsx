/** 工作台一级导航的宽度、收起状态和顶部开关。 */
import { useCallback, useState, type PointerEvent as ReactPointerEvent } from "react"
import { PanelLeftCloseIcon, PanelLeftOpenIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

const railMinWidth = 200
const railDefaultWidth = 220
const railMaxWidth = 300
const widthStorageKey = "cervi.workspace.rail-width"
const collapsedStorageKey = "cervi.workspace.rail-collapsed"

/** 把宽度收敛到可调范围内。 */
function clampRailWidth(width: number) {
  return Math.min(railMaxWidth, Math.max(railMinWidth, Math.round(width)))
}

/** 读取并更新本机记录的一级导航宽度和收起状态。 */
export function useWorkspaceRail() {
  const [width, setWidth] = useState(() => {
    try {
      const stored = Number(localStorage.getItem(widthStorageKey))
      return Number.isFinite(stored) && stored > 0
        ? clampRailWidth(stored)
        : railDefaultWidth
    } catch {
      return railDefaultWidth
    }
  })

  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(collapsedStorageKey) === "true"
    } catch {
      return false
    }
  })

  const changeWidth = useCallback((next: number) => {
    const clamped = clampRailWidth(next)
    setWidth(clamped)
    try {
      localStorage.setItem(widthStorageKey, String(clamped))
    } catch {
      // 本机存储不可用时只在当前页面保留宽度。
    }
  }, [])

  const toggleCollapsed = useCallback(() => {
    setCollapsed((current) => {
      const next = !current
      try {
        localStorage.setItem(collapsedStorageKey, String(next))
      } catch {
        // 本机存储不可用时只在当前页面保留收起状态。
      }
      return next
    })
  }, [])

  return { width, changeWidth, collapsed, toggleCollapsed }
}

/** 标题栏上的一级导航收起开关。 */
export function WorkspaceRailToggle({
  collapsed,
  onToggle,
}: {
  collapsed: boolean
  onToggle: () => void
}) {
  const { t } = useTranslation("workspace")
  const label = collapsed ? t("railOpen") : t("railClose")

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="text-muted-foreground"
          aria-label={label}
          onClick={onToggle}
        >
          {collapsed ? <PanelLeftOpenIcon /> : <PanelLeftCloseIcon />}
        </Button>
      </TooltipTrigger>
      {/* 窄栏内的开关上方是 macOS 窗口按钮，提示改从右侧弹出。 */}
      <TooltipContent side={collapsed ? "right" : undefined}>
        {label}
      </TooltipContent>
    </Tooltip>
  )
}

/** 结束拖动一级导航。 */
function stopRailResize(event: ReactPointerEvent<HTMLButtonElement>) {
  if (event.currentTarget.hasPointerCapture(event.pointerId)) {
    event.currentTarget.releasePointerCapture(event.pointerId)
  }
}

/** 一级导航右边缘的宽度拖动手柄。 */
export function WorkspaceRailResizer({
  onWidthChange,
}: {
  onWidthChange: (width: number) => void
}) {
  const { t } = useTranslation("workspace")

  return (
    <div className="relative h-full min-h-0 w-0 shrink-0">
      <button
        type="button"
        // 手柄居中于一级导航与主内容卡片之间的内缩间隙。
        className="absolute top-0 left-0 z-30 h-full w-2 -translate-x-px cursor-col-resize touch-none"
        aria-label={t("resizeNavigation")}
        onPointerDown={(event) => {
          // 开始拖动一级导航。
          event.preventDefault()
          event.currentTarget.setPointerCapture(event.pointerId)
        }}
        onPointerMove={(event) => {
          if (!event.currentTarget.hasPointerCapture(event.pointerId)) {
            return
          }
          // 一级导航贴着窗口左边缘，指针横坐标即为拖动后的宽度。
          onWidthChange(event.clientX)
        }}
        onPointerUp={stopRailResize}
        onPointerCancel={stopRailResize}
      />
    </div>
  )
}
