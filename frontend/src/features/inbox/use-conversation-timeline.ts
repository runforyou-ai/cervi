/** 管理单个连续消息窗口、资源读取和最新/锚点浏览意图。 */
import { useCallback, useEffect, useRef, useState } from "react"
import {
  getConversationMessageContext,
  listConversationMessages,
  type ConversationMessageListData,
} from "@/api"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { mergeConversationPage } from "./conversation-window"
import { memberChatPollingInterval } from "./use-member-chat-polling"

/** 在统一资源缓存上拼装当前连续窗口，过期读取不影响新窗口。 */
export function useConversationTimeline(
  conversationID: string,
  pollingActive: boolean,
) {
  const [page, setPage] = useState<ConversationMessageListData | null>(null)
  const [mode, setMode] = useState<"latest" | "anchor">("latest")
  const [loadingDirection, setLoadingDirection] = useState<
    "before" | "after" | null
  >(null)
  const [pageError, setPageError] = useState<"before" | "after" | null>(null)
  const invalidate = useResourceInvalidator()
  const [switching, setSwitching] = useState(false)
  const generation = useRef(0)
  const modeRef = useRef(mode)
  const pageRef = useRef(page)
  modeRef.current = mode
  pageRef.current = page
  const initial = useResource(
    resourceKeys.conversationMessages(conversationID),
    (signal) => listConversationMessages(conversationID, undefined, signal),
    { refetchOnWindowFocus: false, staleTime: 0 },
  )
  const { read } = initial
  const after = page?.after ?? ""
  const incoming = useResource(
    resourceKeys.conversationMessagePage(conversationID, { before: "", after }),
    (signal) =>
      listConversationMessages(conversationID, { before: "", after }, signal),
    {
      enabled:
        pollingActive &&
        mode === "latest" &&
        Boolean(page) &&
        !switching &&
        !loadingDirection,
      refetchInterval:
        pollingActive && mode === "latest" && !switching && !loadingDirection
          ? memberChatPollingInterval
          : false,
      refetchOnWindowFocus: false,
    },
  )

  useEffect(() => {
    if (initial.data && !pageRef.current) setPage(initial.data)
  }, [initial.data])
  useEffect(() => {
    const incomingPage = incoming.data
    if (
      !incomingPage ||
      modeRef.current !== "latest" ||
      switching ||
      loadingDirection
    )
      return
    setPage((current) => {
      if (!current || (current.after ?? "") !== after) return current
      if (!incomingPage.messages.length && !current.hasLater) return current
      return mergeConversationPage(current, incomingPage, "after")
    })
  }, [incoming.data, after, switching, loadingDirection])
  useEffect(
    () => () => {
      generation.current += 1
    },
    [conversationID],
  )

  /** 使旧窗口读取失效，保留当前已展示内容。 */
  const cancelWindowUpdate = useCallback(() => {
    generation.current += 1
    setSwitching(false)
    setLoadingDirection(null)
  }, [])

  /** 切换浏览窗口；定位已有消息只改变浏览意图。 */
  const openWindow = useCallback(
    async (messageID?: string) => {
      const revision = ++generation.current
      const previousMode = modeRef.current
      setSwitching(true)
      setLoadingDirection(null)
      setPageError(null)
      try {
        let next = pageRef.current
        if (!messageID)
          next = await read(
            resourceKeys.conversationMessages(conversationID),
            (signal) =>
              listConversationMessages(conversationID, undefined, signal),
          )
        else if (!next?.messages.some((message) => message.id === messageID))
          next = await read(
            resourceKeys.conversationMessageContext(conversationID, messageID),
            (signal) =>
              getConversationMessageContext(conversationID, messageID, signal),
          )
        if (generation.current !== revision) return false
        modeRef.current = messageID ? "anchor" : "latest"
        setMode(modeRef.current)
        setPage(next)
        return true
      } catch (error) {
        if (generation.current !== revision) return false
        setMode(previousMode)
        throw error
      } finally {
        if (generation.current === revision) setSwitching(false)
      }
    },
    [conversationID, read],
  )

  /** 只把匹配当前端点的历史页合入窗口。 */
  const loadPage = useCallback(
    async (direction: "before" | "after") => {
      const base = pageRef.current
      if (
        !base ||
        loadingDirection ||
        switching ||
        !(direction === "before" ? base.hasEarlier : base.hasLater)
      )
        return
      const cursor = base[direction]
      if (!cursor) return
      const revision = generation.current
      setLoadingDirection(direction)
      setPageError(null)
      try {
        const parameters = {
          before: direction === "before" ? cursor : "",
          after: direction === "after" ? cursor : "",
        }
        const next = await read(
          resourceKeys.conversationMessagePage(conversationID, parameters),
          (signal) =>
            listConversationMessages(conversationID, parameters, signal),
        )
        if (generation.current !== revision) return
        setPage((current) =>
          current?.[direction] === cursor
            ? mergeConversationPage(current, next, direction)
            : current,
        )
      } catch (error) {
        if (generation.current === revision) {
          setPageError(direction)
          throw error
        }
      } finally {
        if (generation.current === revision) setLoadingDirection(null)
      }
    },
    [conversationID, read, loadingDirection, switching],
  )

  /** 失效引用立即在当前窗口显示删除状态，并刷新同会话的资源。 */
  function markReferenceUnavailable(messageID: string) {
    setPage((current) =>
      current
        ? {
            ...current,
            messages: current.messages.map((message) =>
              message.replyTo?.id === messageID
                ? {
                    ...message,
                    replyTo: {
                      ...message.replyTo,
                      deleted: true,
                      body: "",
                      sender: null,
                    },
                  }
                : message,
            ),
          }
        : current,
    )
    void invalidate(resourceKeys.conversationMessages(conversationID))
    void invalidate(resourceKeys.conversationMessageContext(conversationID))
    void invalidate(resourceKeys.conversationMessagePage(conversationID))
  }

  return {
    page,
    mode,
    switching,
    cancelWindowUpdate,
    loading: initial.loading,
    error: initial.error,
    refresh: initial.refresh,
    read,
    openWindow,
    loadPage,
    loadingDirection,
    pageError,
    markReferenceUnavailable,
    pollingError: incoming.error,
    poll: incoming.refresh,
  }
}
