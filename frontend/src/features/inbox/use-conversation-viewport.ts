/** 统一管理消息视口的跟随、定位、分页锚点和尺寸变化。 */
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react"
import type { ConversationMessageListData } from "@/api"
import { conversationMessageScrollOffset } from "./conversation-message-visibility"

/** 返回时间线滚动视口。 */
export function conversationViewport(root: HTMLDivElement | null) {
  return root?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]') ?? null
}

/** 用统一阈值判断视口是否已到尾端。 */
function isAtBottom(viewport: HTMLElement) {
  return viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight <= 48
}

/** 让各类导航共享唯一的滚动位置和跟随意图。 */
export function useConversationViewport({
  root, page, mode, switching, visibleCount, sentCount,
}: {
  root: RefObject<HTMLDivElement | null>
  page: ConversationMessageListData | null
  mode: "latest" | "anchor"
  switching: boolean
  visibleCount: number
  sentCount: number
}) {
  const following = useRef(true)
  const firstScroll = useRef(true)
  const locationHold = useRef<boolean | null>(null)
  const lastSentCount = useRef(sentCount)
  const lastPosition = useRef({ top: 0, height: 0, viewportHeight: 0 })
  const prepend = useRef<{ id: string; offset: number } | null>(null)
  const [atBottom, setAtBottom] = useState(true)
  const [positionRevision, setPositionRevision] = useState(0)

  /** 同步实际位置，并记录已处理的程序滚动和布局尺寸。 */
  const readPosition = useCallback(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport) return false
    lastPosition.current = {
      top: viewport.scrollTop,
      height: viewport.scrollHeight,
      viewportHeight: viewport.clientHeight,
    }
    const bottom = isAtBottom(viewport)
    setAtBottom(bottom)
    return bottom
  }, [root])

  /** 跟随时吸收异步布局变化，定位和分页期间保留阅读位置。 */
  const syncPosition = useCallback(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport) return
    if (following.current && mode === "latest" && !switching && locationHold.current === null && !prepend.current)
      viewport.scrollTop = viewport.scrollHeight
    readPosition()
  }, [root, mode, switching, readPosition])

  useLayoutEffect(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport || !page || switching) return
    if (prepend.current) {
      const node = viewport.querySelector<HTMLElement>(`[data-message-id="${prepend.current.id}"]`)
      if (node)
        viewport.scrollTop += node.getBoundingClientRect().top - viewport.getBoundingClientRect().top - prepend.current.offset
      prepend.current = null
    } else if (mode === "latest" && (firstScroll.current || sentCount > lastSentCount.current)) {
      following.current = true
    }
    firstScroll.current = false
    lastSentCount.current = sentCount
    syncPosition()
  }, [page, mode, switching, visibleCount, sentCount, positionRevision, root, syncPosition])

  useEffect(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport) return
    /** 仅把尺寸稳定时的新滚动位置作为用户意图，布局变化继续沿用跟随状态。 */
    const onScroll = () => {
      const previous = lastPosition.current
      if (viewport.scrollHeight !== previous.height || viewport.clientHeight !== previous.viewportHeight)
        syncPosition()
      else {
        if (viewport.scrollTop !== previous.top) {
          locationHold.current = null
          following.current = isAtBottom(viewport)
        }
        readPosition()
      }
    }
    viewport.addEventListener("scroll", onScroll, { passive: true })
    const observer = new ResizeObserver(syncPosition)
    observer.observe(viewport)
    if (viewport.firstElementChild) observer.observe(viewport.firstElementChild)
    return () => {
      viewport.removeEventListener("scroll", onScroll)
      observer.disconnect()
    }
  }, [root, Boolean(page), syncPosition, readPosition])

  /** 定位前保留跟随意图，窗口外目标也需先暂停，避免上下文布局抢先贴底。 */
  const holdForLocation = useCallback(() => {
    locationHold.current = following.current
  }, [])

  /** 取消或失败时恢复尚未完成的定位意图，已定位的消息保持原位。 */
  const releaseLocation = useCallback(() => {
    if (locationHold.current === null) return
    following.current = locationHold.current
    locationHold.current = null
    setPositionRevision((revision) => revision + 1)
  }, [])

  /** 可见目标保持原位，其余目标定位后按实际位置决定是否跟随。 */
  const revealMessage = useCallback((id: string) => {
    const viewport = conversationViewport(root.current)
    const row = viewport?.querySelector<HTMLElement>(`[data-message-id="${id}"]`)
    if (!viewport || !row) return false
    viewport.scrollTop += conversationMessageScrollOffset(row.getBoundingClientRect(), viewport.getBoundingClientRect())
    row.focus({ preventScroll: true })
    locationHold.current = null
    following.current = readPosition()
    return true
  }, [root, readPosition])

  /** 合入相邻页前保存首个可见消息的像素偏移。 */
  const preservePosition = useCallback(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport) return
    const top = viewport.getBoundingClientRect().top
    const node = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")]
      .find((row) => row.getBoundingClientRect().bottom > top)
    if (node?.dataset.messageId) {
      following.current = false
      prepend.current = { id: node.dataset.messageId, offset: node.getBoundingClientRect().top - top }
    }
  }, [root])

  /** 返回最新窗口时立即贴底，并持续跟随最终布局。 */
  const followLatest = useCallback(() => {
    prepend.current = null
    locationHold.current = null
    following.current = true
    firstScroll.current = true
    const viewport = conversationViewport(root.current)
    if (viewport) viewport.scrollTop = viewport.scrollHeight
    readPosition()
    setPositionRevision((revision) => revision + 1)
  }, [root, readPosition])

  /** 阅读判定直接读取当前布局，避免使用上一帧的位置状态。 */
  const getAtBottom = useCallback(() => {
    const viewport = conversationViewport(root.current)
    return Boolean(viewport && isAtBottom(viewport))
  }, [root])

  return { atBottom, getAtBottom, holdForLocation, releaseLocation, revealMessage, preservePosition, followLatest }
}
