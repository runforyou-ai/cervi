/** 消息页中栏搜索模式的状态、检索读取与键盘选择。 */
import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react"

import {
  InboxScope,
  InboxSearchRange,
  readInboxConversations,
  searchInbox,
  type InboxConversation,
  type InboxQuery,
  type InboxSearchResultData,
} from "@/api"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

const searchInputDelay = 200

export type InboxSearchType = "all" | "conversations" | "messages" | "people"
export type InboxSearchMessageData = InboxSearchResultData["messages"][number]
export type InboxSearchPersonData = InboxSearchResultData["people"][number]

/** 搜索模式中可用键盘选择和打开的结果项。 */
export type InboxSearchItem =
  | { kind: "conversation"; conversation: InboxConversation }
  | { kind: "message"; message: InboxSearchMessageData }
  | { kind: "person"; person: InboxSearchPersonData }

export type InboxSearchState = ReturnType<typeof useInboxSearch>

/** 管理搜索模式、按范围读取结果和最近打开会话，并提供跨分组的键盘选择。 */
export function useInboxSearch({
  query,
  recentConversationIds,
  onOpen,
}: {
  query: InboxQuery
  recentConversationIds: string[]
  onOpen: (item: InboxSearchItem) => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const activeRef = useRef(false)
  const [active, setActive] = useState(false)
  const [text, setText] = useState("")
  const [searchedText, setSearchedText] = useState("")
  const [selectedRange, setRange] = useState<InboxSearchRange>(InboxSearchRange.InboxSearchRangeReadable)
  const [conversationId, setConversationId] = useState("")
  const [type, setType] = useState<InboxSearchType>("all")
  const [activeIndex, setActiveIndex] = useState(0)
  const listRange = query.scope !== InboxScope.InboxScopeAll
  const trimmedText = text.trim()
  // 当前范围为「全部」时列表筛选与全部消息等价，只保留全部消息。
  const range =
    (selectedRange === InboxSearchRange.InboxSearchRangeList && !listRange) ||
    (selectedRange === InboxSearchRange.InboxSearchRangeConversation && !conversationId)
      ? InboxSearchRange.InboxSearchRangeReadable
      : selectedRange
  const conversationRange = range === InboxSearchRange.InboxSearchRangeConversation

  useEffect(() => {
    // 输入停顿后再检索。
    const timer = window.setTimeout(() => setSearchedText(text.trim()), searchInputDelay)
    return () => window.clearTimeout(timer)
  }, [text])

  const searchParameters = {
    query: searchedText,
    range,
    conversationId: conversationRange ? conversationId : "",
    ...(range === InboxSearchRange.InboxSearchRangeList ? query : {}),
  }
  const results = useResource(
    resourceKeys.inboxSearch(searchParameters),
    (signal) => searchInbox(searchParameters, signal),
    { enabled: active && searchedText !== "", staleTime: 0 },
  )
  const showRecent = active && trimmedText === "" && !conversationRange
  const recent = useResource(
    resourceKeys.recentConversations({ query, conversationIds: recentConversationIds }),
    (signal) => readInboxConversations({ query, conversationIds: recentConversationIds }, signal),
    { enabled: showRecent && recentConversationIds.length > 0, staleTime: 0 },
  )

  // 只展示与当前输入一致的检索结果。
  const current = active && trimmedText !== "" && trimmedText === searchedText
  const data = current ? results.data : undefined
  const recentConversations = showRecent
    ? (recent.data?.results ?? []).flatMap((result) => (result.conversation ? [result.conversation] : []))
    : []
  const conversations = data && !conversationRange && (type === "all" || type === "conversations") ? data.conversations : []
  const messages = data && (conversationRange || type === "all" || type === "messages") ? data.messages : []
  const people = data && !conversationRange && (type === "all" || type === "people") ? data.people : []
  const items: InboxSearchItem[] = [
    ...(showRecent ? recentConversations : conversations).map((conversation) => ({ kind: "conversation" as const, conversation })),
    ...messages.map((message) => ({ kind: "message" as const, message })),
    ...people.map((person) => ({ kind: "person" as const, person })),
  ]
  const selectedIndex = Math.min(activeIndex, items.length - 1)

  useEffect(() => {
    setActiveIndex(0)
  }, [searchedText, type, range, showRecent])

  const enter = useCallback(
    (targetConversationId = "") => {
      activeRef.current = true
      setActive(true)
      setConversationId(targetConversationId)
      setRange(
        targetConversationId
          ? InboxSearchRange.InboxSearchRangeConversation
          : listRange
            ? InboxSearchRange.InboxSearchRangeList
            : InboxSearchRange.InboxSearchRangeReadable,
      )
      setType("all")
      inputRef.current?.focus()
    },
    [listRange],
  )

  const exit = useCallback(() => {
    activeRef.current = false
    setActive(false)
    setText("")
    setSearchedText("")
    setConversationId("")
    setType("all")
    inputRef.current?.blur()
  }, [])

  useEffect(() => {
    // Ctrl/⌘ K 在消息页任意位置进入搜索模式；搜索模式下焦点不在其他输入控件或浮层时，Esc 退出搜索。
    function handleShortcut(event: globalThis.KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === "k") {
        event.preventDefault()
        if (activeRef.current) inputRef.current?.focus()
        else enter()
        return
      }
      if (event.key !== "Escape" || event.isComposing || event.defaultPrevented || !activeRef.current) return
      const focused = document.activeElement
      if (
        focused instanceof HTMLElement &&
        focused !== inputRef.current &&
        (focused.isContentEditable ||
          ["INPUT", "TEXTAREA", "SELECT"].includes(focused.tagName) ||
          focused.closest("[data-radix-popper-content-wrapper],[role='dialog']"))
      )
        return
      event.preventDefault()
      exit()
    }
    window.addEventListener("keydown", handleShortcut)
    return () => window.removeEventListener("keydown", handleShortcut)
  }, [enter, exit])

  /** 聚焦搜索框时进入搜索模式，已在搜索模式时保留当前范围。 */
  function handleFocus() {
    if (!activeRef.current) enter()
  }

  /** 在搜索框内用方向键跨分组选择，Enter 打开；Esc 由文档级快捷键统一处理。 */
  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.nativeEvent.isComposing) return
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault()
      if (items.length === 0) return
      const step = event.key === "ArrowDown" ? 1 : -1
      setActiveIndex((selectedIndex + step + items.length) % items.length)
    } else if (event.key === "Enter" && items[selectedIndex]) {
      event.preventDefault()
      onOpen(items[selectedIndex])
    }
  }

  return {
    inputRef,
    active,
    text,
    setText,
    range,
    setRange,
    listRange,
    conversationId,
    type,
    setType,
    showRecent,
    recentConversations,
    conversations,
    messages,
    people,
    items,
    selectedIndex,
    setActiveIndex,
    pending: trimmedText !== "" && (!current || results.loading),
    error: current ? results.error : null,
    retry: results.refresh,
    open: onOpen,
    enter,
    exit,
    handleFocus,
    handleKeyDown,
  }
}
