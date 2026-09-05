/** 统一消息定位、可见确认及整行高亮，隔离过期跳转。 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type RefObject,
} from "react"
import type { ConversationMessageListData } from "@/api"

type PendingLocation = {
  id: string
  revision: number
  resolve: (visible: boolean) => void
}

/** 返回时间线滚动视口。 */
export function conversationViewport(root: HTMLDivElement | null) {
  return (
    root?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]') ??
    null
  )
}

/** 定位消息并在当前窗口实际可见时完成确认。 */
export function useConversationMessageNavigation({
  root,
  page,
  readingActive,
  openWindow,
  cancelWindowUpdate,
}: {
  root: RefObject<HTMLDivElement | null>
  page: ConversationMessageListData | null
  readingActive: boolean
  openWindow: (messageID?: string) => Promise<boolean>
  cancelWindowUpdate: () => void
}) {
  const revision = useRef(0)
  const pending = useRef<PendingLocation | null>(null)
  const positioned = useRef(0)
  const highlightTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [target, setTarget] = useState<{ id: string; revision: number } | null>(
    null,
  )
  const [highlightedID, setHighlightedID] = useState<string | null>(null)
  const [locating, setLocating] = useState(false)

  /** 结束未完成的定位并清理旧高亮。 */
  const cancel = useCallback(() => {
    revision.current += 1
    pending.current?.resolve(false)
    pending.current = null
    if (highlightTimer.current !== null) clearTimeout(highlightTimer.current)
    cancelWindowUpdate()
    setTarget(null)
    setHighlightedID(null)
    setLocating(false)
  }, [cancelWindowUpdate])

  /** 用统一窗口入口定位目标，等待实际视口确认。 */
  const locate = useCallback(
    async (id: string) => {
      cancel()
      const current = revision.current
      setLocating(true)
      try {
        if (!(await openWindow(id)) || revision.current !== current)
          return false
        return await new Promise<boolean>((resolve) => {
          pending.current = { id, revision: current, resolve }
          setTarget({ id, revision: current })
        })
      } finally {
        if (revision.current === current) setLocating(false)
      }
    },
    [cancel, openWindow],
  )

  useLayoutEffect(() => {
    if (!target || !readingActive || positioned.current === target.revision)
      return
    const viewport = conversationViewport(root.current)
    const row = viewport?.querySelector<HTMLElement>(
      `[data-message-id="${target.id}"]`,
    )
    if (!viewport || !row) return
    const rect = row.getBoundingClientRect()
    viewport.scrollTop +=
      rect.top -
      viewport.getBoundingClientRect().top -
      Math.max(0, (viewport.clientHeight - rect.height) / 2)
    if (readingActive) row.focus({ preventScroll: true })
    const frame = requestAnimationFrame(() => {
      if (revision.current !== target.revision) return
      positioned.current = target.revision
      setHighlightedID(target.id)
      highlightTimer.current = setTimeout(() => setHighlightedID(null), 2000)
    })
    return () => cancelAnimationFrame(frame)
  }, [target, page, readingActive, root])

  useEffect(() => {
    if (!target || !readingActive) return
    const viewport = conversationViewport(root.current)
    const row = viewport?.querySelector<HTMLElement>(
      `[data-message-id="${target.id}"]`,
    )
    if (!viewport || !row) return
    /** 只确认仍属于当前跳转且正文可见的目标。 */
    function checkVisible() {
      if (!viewport || !row || pending.current?.revision !== target?.revision)
        return
      const bounds = viewport.getBoundingClientRect()
      const rect = row.getBoundingClientRect()
      const visible =
        Math.min(bounds.bottom, rect.bottom) - Math.max(bounds.top, rect.top)
      if (visible < Math.min(rect.height / 2, 32)) return
      pending.current?.resolve(true)
      pending.current = null
    }
    const observer = new IntersectionObserver(checkVisible, {
      root: viewport,
      threshold: [0, 0.25, 0.5, 1],
    })
    observer.observe(row)
    checkVisible()
    return () => observer.disconnect()
  }, [target, page, readingActive, root])

  useEffect(
    () => () => {
      revision.current += 1
      pending.current?.resolve(false)
      if (highlightTimer.current !== null) clearTimeout(highlightTimer.current)
    },
    [],
  )
  return { locate, cancel, locating, highlightedID }
}
