/** macOS 原生窗口的透明标题栏拖拽区域。 */

/** 渲染仅在 macOS 普通窗口中可见的顶部拖拽区域。 */
export function WindowDragRegion() {
  return <div aria-hidden="true" className="cervi-window-drag-region" />
}
