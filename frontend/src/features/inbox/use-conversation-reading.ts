/** 保持时间线滚动位置，并只在最新窗口中推进普通已读。 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type RefObject,
} from "react"
import { MessageType, type ConversationMessageListData } from "@/api"
import { conversationViewport } from "./use-conversation-message-navigation"

/** 协调贴底、前插锚点和连续可见已读。 */
export function useConversationReading({
  root,
  page,
  mode,
  switching,
  readingActive,
  identityID,
  visibleCount,
  sentCount,
  onReadMessage,
  readThroughMessageID,
}: {
  root: RefObject<HTMLDivElement | null>
  page: ConversationMessageListData | null
  mode: "latest" | "anchor"
  switching: boolean
  readingActive: boolean
  identityID: string
  visibleCount: number
  sentCount: number
  onReadMessage?: (id: string) => void
  readThroughMessageID?: string | null
}) {
  const nearBottom = useRef(true)
  const firstScroll = useRef(true)
  const readID = useRef(readThroughMessageID ?? "")
  const queuedID = useRef("")
  const seen = useRef(new Set<string>())
  const previous = useRef(new Set<string>())
  const newMessages = useRef(new Set<string>())
  const readTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const prepend = useRef<{
    id: string
    offset: number
    page: ConversationMessageListData | null
  } | null>(null)
  const lastSentCount = useRef(sentCount)
  const [newCount, setNewCount] = useState(0)
  const [followRevision, setFollowRevision] = useState(0)
  const [atBottom, setAtBottom] = useState(true)

  /** 合并同一窗口连续产生的已读写入。 */
  const queueRead = useCallback(
    (id: string, immediate = false) => {
      if (!onReadMessage || id === readID.current || id === queuedID.current)
        return
      queuedID.current = id
      if (readTimer.current) clearTimeout(readTimer.current)
      if (immediate) {
        readID.current = id
        queuedID.current = ""
        onReadMessage(id)
      } else
        readTimer.current = setTimeout(() => {
          readID.current = id
          queuedID.current = ""
          onReadMessage(id)
        }, 250)
    },
    [onReadMessage],
  )

  useEffect(() => {
    if (!readID.current && readThroughMessageID)
      readID.current = readThroughMessageID
  }, [readThroughMessageID])
  useEffect(() => {
    if (mode === "anchor" || switching || !readingActive) {
      if (readTimer.current) clearTimeout(readTimer.current)
      queuedID.current = ""
    }
  }, [mode, switching, readingActive])
  useEffect(
    () => () => {
      if (readTimer.current) clearTimeout(readTimer.current)
    },
    [],
  )

  /** 按视口实际尺寸同步位置，覆盖滚动和消息内容高度变化。 */
  const syncViewportPosition = useCallback(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport) return
    const bottom =
      viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight <= 48
    setAtBottom(bottom)
    return bottom
  }, [root])

  useEffect(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport) return
    syncViewportPosition()
    // 仅滚动更新贴底跟随意图，尺寸变化只同步按钮状态。
    const onScroll = () => {
      nearBottom.current = syncViewportPosition() ?? nearBottom.current
    }
    viewport.addEventListener("scroll", onScroll, { passive: true })
    const observer = new ResizeObserver(syncViewportPosition)
    observer.observe(viewport)
    if (viewport.firstElementChild) observer.observe(viewport.firstElementChild)
    return () => {
      viewport.removeEventListener("scroll", onScroll)
      observer.disconnect()
    }
  }, [root, Boolean(page), syncViewportPosition])

  useLayoutEffect(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport || !page) return
    if (switching) {
      prepend.current = null
      return
    }
    if (prepend.current?.page === page) return
    const prepending = Boolean(prepend.current)
    if (prepend.current) {
      const node = viewport.querySelector<HTMLElement>(
        `[data-message-id="${prepend.current.id}"]`,
      )
      if (node)
        viewport.scrollTop +=
          node.getBoundingClientRect().top -
          viewport.getBoundingClientRect().top -
          prepend.current.offset
      prepend.current = null
    } else if (
      mode === "latest" &&
      !switching &&
      (firstScroll.current ||
        nearBottom.current ||
        sentCount > lastSentCount.current)
    ) {
      viewport.scrollTop = viewport.scrollHeight
      nearBottom.current = true
      firstScroll.current = false
      newMessages.current.clear()
      setNewCount(0)
    }
    syncViewportPosition()
    if (
      mode === "latest" &&
      !switching &&
      readingActive &&
      nearBottom.current &&
      !page.hasLater
    ) {
      const latest = page.messages[page.messages.length - 1]
      if (latest) queueRead(latest.id, true)
    }
    if (
      !prepending &&
      mode === "latest" &&
      !nearBottom.current &&
      previous.current.size
    ) {
      for (const message of page.messages) {
        if (
          !previous.current.has(message.id) &&
          (message.type === MessageType.MessageTypeSystem ||
            message.sender?.sourceId !== identityID)
        )
          newMessages.current.add(message.id)
      }
      setNewCount(newMessages.current.size)
    }
    previous.current = new Set(page.messages.map((message) => message.id))
    lastSentCount.current = sentCount
  }, [
    page,
    mode,
    switching,
    readingActive,
    root,
    visibleCount,
    sentCount,
    identityID,
    queueRead,
    followRevision,
    syncViewportPosition,
  ])

  useEffect(() => {
    const viewport = conversationViewport(root.current)
    if (
      !viewport ||
      !page ||
      mode !== "latest" ||
      switching ||
      !readingActive ||
      !onReadMessage
    )
      return
    const messages = page.messages
    const byID = new Map(messages.map((message) => [message.id, message]))
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          const id = (entry.target as HTMLElement).dataset.messageId
          if (
            !id ||
            !byID.has(id) ||
            !entry.isIntersecting ||
            entry.intersectionRect.height <
              Math.min(entry.boundingClientRect.height / 2, 32)
          )
            continue
          seen.current.add(id)
          newMessages.current.delete(id)
        }
        setNewCount(newMessages.current.size)
        const index = messages.findIndex(
          (message) => message.id === readID.current,
        )
        if (readID.current && index < 0) return
        let nextID = readID.current
        for (const message of messages.slice(index + 1)) {
          if (
            message.sender?.sourceId !== identityID &&
            !seen.current.has(message.id)
          )
            break
          nextID = message.id
        }
        if (nextID) queueRead(nextID)
      },
      { root: viewport, threshold: [0, 0.25, 0.5, 1] },
    )
    for (const node of viewport.querySelectorAll("[data-message-id]"))
      observer.observe(node)
    return () => observer.disconnect()
  }, [
    root,
    page,
    mode,
    switching,
    readingActive,
    identityID,
    onReadMessage,
    queueRead,
  ])

  /** 前插消息前保存首个可见消息的像素偏移。 */
  const preservePosition = useCallback(() => {
    const viewport = conversationViewport(root.current)
    if (!viewport) return
    const top = viewport.getBoundingClientRect().top
    const node = [
      ...viewport.querySelectorAll<HTMLElement>("[data-message-id]"),
    ].find((row) => row.getBoundingClientRect().bottom > top)
    if (node?.dataset.messageId)
      prepend.current = {
        id: node.dataset.messageId,
        offset: node.getBoundingClientRect().top - top,
        page,
      }
  }, [root, page])

  /** 分页失败后丢弃尚未应用的滚动锚点。 */
  const cancelPreservedPosition = useCallback(() => {
    prepend.current = null
  }, [])

  /** 返回最新窗口后明确恢复贴底。 */
  const followLatest = useCallback(() => {
    firstScroll.current = true
    nearBottom.current = true
    setFollowRevision((current) => current + 1)
  }, [])
  return {
    newCount,
    atBottom,
    preservePosition,
    cancelPreservedPosition,
    followLatest,
  }
}
