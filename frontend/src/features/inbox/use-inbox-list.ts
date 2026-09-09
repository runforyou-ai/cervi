/** 将列表控制器接入统一 Query 缓存、前台轮询和会话资源清理。 */
import { useEffect, useMemo, useRef, useSyncExternalStore } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { getInboxContext, loadInbox, readInboxConversations, readInboxWindow, type InboxQuery, type InboxConversationResults } from "@/api"
import { useWorkspace } from "@/contexts/workspace-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { useAttachmentQueue } from "./attachment-queue-context"
import { clearConversationResources } from "./conversation-resources"
import { InboxListController, normalizeInboxListQuery, type InboxListPorts } from "./inbox-list-controller"
import { memberChatPollingInterval, useMemberChatPollingActive } from "./use-member-chat-polling"

export type InboxListViewport = Pick<InboxListPorts, "capture" | "atTop" | "interacting" | "restore">

/** 每个查询持有独立浏览状态，业务摘要仅从当前批量 Query 读取。 */
export function useInboxList(input: InboxQuery, viewport: InboxListViewport) {
  const { identity, beginUnreadSnapshot, applyUnreadSnapshot } = useWorkspace()
  const { queue } = useAttachmentQueue()
  const client = useQueryClient()
  const active = useMemberChatPollingActive()
  const scope = normalizeInboxListQuery(input)
  const query = useMemo(() => scope, [scope.scope, scope.customerView, scope.assigneeIdentityId])
  const owner = useMemo(() => ({ organizationId: identity.organization.id, userId: identity.user.id }), [identity.organization.id, identity.user.id])
  const headKey = resourceKeys.inbox({ ...owner, ...query })
  const resource = useResource(headKey, () => loadInbox(query), { enabled: false })
  const { read } = resource
  const callbacks = useRef({ viewport, queue, beginUnreadSnapshot, applyUnreadSnapshot })
  callbacks.current = { viewport, queue, beginUnreadSnapshot, applyUnreadSnapshot }
  const controller = useMemo(() => new InboxListController({
    page: (cursor = "", beforeCursor = "") => read(
      resourceKeys.inbox({ ...owner, ...query, ...(cursor || beforeCursor ? { cursor, beforeCursor } : {}) }),
      () => loadInbox({ ...query, cursor, beforeCursor }),
    ),
    window: (startCursor, endCursor) => read(
      resourceKeys.inboxWindow({ ...owner, query, startCursor, endCursor }),
      () => readInboxWindow({ query, startCursor, endCursor }),
    ),
    context: async (anchor) => {
      const parameters = { query, anchorId: anchor.id, anchorCursor: anchor.cursor, beforeLimit: 25, afterLimit: 25 }
      return (await read(resourceKeys.inboxContext({ ...owner, ...parameters }), () => getInboxContext(parameters))).window
    },
    rows: (conversationIds) => read(
      resourceKeys.inboxConversations({ ...owner, query, conversationIds }),
      (signal) => readInboxConversations({ query, conversationIds }, signal),
    ),
    capture: () => callbacks.current.viewport.capture(),
    atTop: () => callbacks.current.viewport.atTop(),
    interacting: () => callbacks.current.viewport.interacting(),
    restore: (...args) => callbacks.current.viewport.restore(...args),
    unavailable: (ids) => {
      for (const id of ids) {
        callbacks.current.queue?.forgetConversation(id)
        clearConversationResources(client, id)
        void client.resetQueries({ queryKey: resourceKeys.conversationSummary(id) })
      }
    },
    unread: (count) => {
      const current = callbacks.current
      current.applyUnreadSnapshot(count, current.beginUnreadSnapshot())
    },
  }, query), [client, owner, query, read])
  const state = useSyncExternalStore(controller.subscribe, controller.getSnapshot)
  const rows = useResource(
    resourceKeys.inboxConversations({ ...owner, query, conversationIds: state.rowIds }),
    (signal) => readInboxConversations({ query, conversationIds: state.rowIds }, signal),
    { enabled: false },
  )

  useEffect(() => {
    void controller.request("initial")
    return () => controller.dispose()
  }, [controller])
  useEffect(() => client.getQueryCache().subscribe((event) => {
    const key = event.query.queryKey
    if (event.type === "updated" && key[0] === resourceKeys.conversationSummary()[0] && event.query.state.data === null) {
      controller.removeUnavailable(String(key[1]))
    }
    // 新批次接管展示后移除含失权摘要的旧缓存，避免其再次被页签复用。
    if (event.type === "observerRemoved" && key[0] === resourceKeys.inboxConversations()[0] && event.query.getObserversCount() === 0) {
      const results = (event.query.state.data as InboxConversationResults | undefined)?.results
      if (results?.some((row) => row.conversation && controller.getSnapshot().unavailableIds.includes(row.id))) client.removeQueries({ queryKey: key, exact: true })
    }
  }), [client, controller])
  useEffect(() => {
    if (!active) return
    void controller.request("poll")
    const timer = window.setInterval(() => void controller.request("poll"), memberChatPollingInterval)
    // 业务变更失效原收件箱入口时，仍经过同一窗口队列重读。
    const unsubscribe = client.getQueryCache().subscribe((event) => {
      if (event.type !== "updated") return
      const prefix = event.query.queryKey[0]
      if ((prefix === resourceKeys.inbox()[0] || prefix === resourceKeys.inboxConversations()[0]) && (event.action.type === "invalidate" || event.action.type === "setState")) void controller.request("poll")
    })
    return () => { window.clearInterval(timer); unsubscribe() }
  }, [active, client, controller])

  const conversations = new Map(rows.data?.results.flatMap((row) => row.availability === "matching" && row.conversation ? [[row.id, row.conversation] as const] : []) ?? [])
  return {
    ...state,
    conversations: state.ids.flatMap((id) => conversations.has(id) ? [conversations.get(id)!] : []),
    request: controller.request,
    retry: controller.retry,
  }
}

export type InboxList = ReturnType<typeof useInboxList>
