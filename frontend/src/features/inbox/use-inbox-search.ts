/** 收件箱检索结果与会话名称搜索分页查询，以及消息页中栏搜索模式的状态与键盘选择。 */
import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react"

import {
  CustomerInboxView,
  InboxPartition,
  InboxScope,
  InboxSearchRange,
  ServiceSessionStatus,
  readInboxConversations,
  searchInbox,
  type InboxConversation,
  type InboxQuery,
  type InboxSearchResultData,
} from "@/api"
import { normalizeInboxQuery, type InboxQueryInput } from "@/features/inbox/inbox-query"
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

/** 按检索文本、范围和类型读取分组结果与最近打开会话，只返回与当前输入一致的结果；会话类型给出名称搜索分页查询。 */
export function useInboxSearchResults({
  active,
  text,
  range: selectedRange,
  conversationId,
  type,
  query,
  recentConversationIds,
}: {
  active: boolean
  text: string
  range: InboxSearchRange
  conversationId: string
  type: InboxSearchType
  query: InboxQueryInput
  recentConversationIds: string[]
}) {
  const trimmedText = text.trim()
  const [searchedText, setSearchedText] = useState(trimmedText)
  const listRange = query.scope !== InboxScope.InboxScopeAll
  // 当前范围为「全部」时列表筛选与全部消息等价，只保留全部消息。
  const range =
    (selectedRange === InboxSearchRange.InboxSearchRangeList && !listRange) ||
    (selectedRange === InboxSearchRange.InboxSearchRangeConversation && !conversationId)
      ? InboxSearchRange.InboxSearchRangeReadable
      : selectedRange
  const conversationRange = range === InboxSearchRange.InboxSearchRangeConversation
  // 会话类型下按名称搜索分页读取全部命中会话，不读取分组检索结果。
  const paged = !conversationRange && type === "conversations"

  useEffect(() => {
    // 输入停顿后再检索；清空输入立即结束上一次检索。
    if (text.trim() === "") {
      setSearchedText("")
      return
    }
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
    { enabled: active && searchedText !== "" && !paged, staleTime: 0 },
  )
  // 名称搜索沿用检索范围：列表范围带当前筛选，可读范围不带其他列表筛选。
  const nameQuery = normalizeInboxQuery({
    ...(range === InboxSearchRange.InboxSearchRangeList
      ? query
      : {
          scope: InboxScope.InboxScopeAll,
          customerView: CustomerInboxView.CustomerInboxViewQueue,
          assigneeIdentityId: "",
          channelId: "",
          serviceStatus: ServiceSessionStatus.ServiceSessionStatusOpen,
          kinds: [],
        }),
    partition: InboxPartition.InboxPartitionAll,
    search: searchedText,
    searchRange: range,
  })
  const showRecent = active && trimmedText === "" && !conversationRange
  // 最近会话只核对当前筛选下的列表资格，不受置顶分区限制。
  const recentQuery: InboxQuery = normalizeInboxQuery({ ...query, partition: InboxPartition.InboxPartitionAll })
  const recent = useResource(
    resourceKeys.recentConversations({ query: recentQuery, conversationIds: recentConversationIds }),
    (signal) => readInboxConversations({ query: recentQuery, conversationIds: recentConversationIds }, signal),
    { enabled: showRecent && recentConversationIds.length > 0, staleTime: 0 },
  )

  // 只展示与当前输入一致的检索结果。
  const current = active && trimmedText !== "" && trimmedText === searchedText
  const data = current && !paged ? results.data : undefined
  return {
    listRange,
    range,
    searchedText,
    showRecent,
    paged: active && paged && trimmedText !== "",
    nameQuery,
    recentConversations: showRecent
      ? (recent.data?.results ?? []).flatMap((result) => (result.conversation ? [result.conversation] : []))
      : [],
    conversations: data && !conversationRange && (type === "all" || type === "conversations") ? data.conversations : [],
    messages: data && (conversationRange || type === "all" || type === "messages") ? data.messages : [],
    people: data && !conversationRange && (type === "all" || type === "people") ? data.people : [],
    pending: trimmedText !== "" && (!current || (!paged && results.loading)),
    error: current && !paged ? results.error : null,
    retry: results.refresh,
  }
}

/** 管理搜索模式、按范围读取结果和最近打开会话，并提供跨分组的键盘选择。 */
export function useInboxSearch({
  query,
  recentConversationIds,
  onOpen,
}: {
  query: InboxQueryInput
  recentConversationIds: string[]
  onOpen: (item: InboxSearchItem) => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const activeRef = useRef(false)
  const [active, setActive] = useState(false)
  const [text, setText] = useState("")
  const [selectedRange, setRange] = useState<InboxSearchRange>(InboxSearchRange.InboxSearchRangeReadable)
  const [conversationId, setConversationId] = useState("")
  const [type, setType] = useState<InboxSearchType>("all")
  const [activeIndex, setActiveIndex] = useState(0)
  const [pagedConversations, setPagedConversations] = useState<InboxConversation[]>([])
  const typeIndexes = useRef(new Map<InboxSearchType, number>())
  const results = useInboxSearchResults({
    active,
    text,
    range: selectedRange,
    conversationId,
    type,
    query,
    recentConversationIds,
  })
  const { listRange, range, searchedText, showRecent } = results
  const items: InboxSearchItem[] = [
    ...(showRecent ? results.recentConversations : results.paged ? pagedConversations : results.conversations).map((conversation) => ({
      kind: "conversation" as const,
      conversation,
    })),
    ...results.messages.map((message) => ({ kind: "message" as const, message })),
    ...results.people.map((person) => ({ kind: "person" as const, person })),
  ]
  const selectedIndex = Math.min(activeIndex, items.length - 1)

  useEffect(() => {
    typeIndexes.current.clear()
    setActiveIndex(0)
  }, [searchedText, range, showRecent])

  /** 修改检索词；处于会话分页时回到分组结果。 */
  const changeText = useCallback(
    (value: string) => {
      setText(value)
      if (type === "conversations") {
        typeIndexes.current.clear()
        setType("all")
      }
    },
    [type],
  )

  /** 切换结果类型，回到查看过的类型时恢复其原选中项。 */
  const selectType = useCallback(
    (next: InboxSearchType) => {
      typeIndexes.current.set(type, selectedIndex)
      setType(next)
      setActiveIndex(typeIndexes.current.get(next) ?? 0)
    },
    [type, selectedIndex],
  )

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
    setConversationId("")
    setType("all")
    typeIndexes.current.clear()
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
    setText: changeText,
    range,
    setRange,
    listRange,
    conversationId,
    type,
    setType: selectType,
    showRecent,
    paged: results.paged,
    nameQuery: results.nameQuery,
    setPagedConversations,
    recentConversations: results.recentConversations,
    conversations: results.conversations,
    messages: results.messages,
    people: results.people,
    items,
    selectedIndex,
    setActiveIndex,
    pending: results.pending,
    error: results.error,
    retry: results.retry,
    open: onOpen,
    enter,
    exit,
    handleFocus,
    handleKeyDown,
  }
}
