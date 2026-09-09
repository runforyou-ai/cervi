/** 保存列表可见邻域，并作为唯一滚动补偿所有者恢复像素位置。 */
import { useEffect, useLayoutEffect, useMemo, useRef } from "react"
import type { InboxListAnchor, InboxListState } from "./inbox-list-controller"

/** 捕获首个可见行及原后继、前驱，DOM 提交后只进行一次位置补偿。 */
export function useInboxListViewport() {
  const root = useRef<HTMLDivElement>(null)
  const positions = useRef<InboxListState["positions"]>([])
  const pending = useRef<{ anchor: InboxListAnchor | null; moved: Set<string>; top: boolean } | null>(null)
  const interaction = useRef({ scrollingUntil: 0, pointer: false, menu: false })
  const viewport = useMemo(() => ({
    root,
    positions,
    interaction,
    capture: (): InboxListAnchor | null => {
      const container = root.current?.querySelector<HTMLElement>("[data-slot=scroll-area-viewport]")
      if (!container || !container.clientHeight) return null
      const top = container.getBoundingClientRect().top
      const rows = [...container.querySelectorAll<HTMLElement>("[data-inbox-id]")]
      const first = rows.findIndex((row) => row.getBoundingClientRect().bottom > top)
      if (first < 0) return null
      const id = rows[first].dataset.inboxId!
      const neighbors = [...rows.slice(first), ...rows.slice(0, first).reverse()].map((row) => ({ id: row.dataset.inboxId!, offset: row.getBoundingClientRect().top - top }))
      return { id, cursor: positions.current.find((row) => row.id === id)?.positionCursor ?? "", neighbors }
    },
    atTop: () => (root.current?.querySelector<HTMLElement>("[data-slot=scroll-area-viewport]")?.scrollTop ?? 0) <= 2,
    interacting: () => interaction.current.pointer || interaction.current.menu || performance.now() < interaction.current.scrollingUntil,
    restore: (anchor: InboxListAnchor | null, moved: Set<string>, top: boolean) => { pending.current = { anchor, moved, top } },
  }), [])

  /** 应用最近一次窗口提交的滚动意图，隐藏容器保留待恢复位置。 */
  function apply() {
    const intent = pending.current
    const container = root.current?.querySelector<HTMLElement>("[data-slot=scroll-area-viewport]")
    if (!intent || !container?.clientHeight) return
    pending.current = null
    if (intent.top) { container.scrollTop = 0; return }
    const top = container.getBoundingClientRect().top
    const rows = new Map([...container.querySelectorAll<HTMLElement>("[data-inbox-id]")].map((row) => [row.dataset.inboxId!, row]))
    const neighbor = intent.anchor?.neighbors.find((row) => !intent.moved.has(row.id) && rows.has(row.id))
    if (neighbor) container.scrollTop += rows.get(neighbor.id)!.getBoundingClientRect().top - top - neighbor.offset
  }

  useEffect(() => {
    // 指针在列表外释放或窗口失焦时也结束拖动保护。
    const release = () => { interaction.current.pointer = false }
    window.addEventListener("pointerup", release)
    window.addEventListener("pointercancel", release)
    window.addEventListener("blur", release)
    return () => {
      window.removeEventListener("pointerup", release)
      window.removeEventListener("pointercancel", release)
      window.removeEventListener("blur", release)
    }
  }, [])
  useLayoutEffect(() => { apply() })
  return viewport
}
