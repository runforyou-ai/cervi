/** 串行调度收件箱窗口读取，只保存列表顺序、查询边界和浏览状态。 */
import type { InboxConversation, InboxQuery, InboxWindow } from "@/api"
import type { loadInbox, readInboxConversations } from "@/api"

export type InboxListOperation = "initial" | "before" | "after" | "poll" | "refresh" | "latest"
export type InboxListAnchor = {
  id: string
  cursor: string
  neighbors: { id: string; offset: number }[]
}
type ListPosition = Pick<InboxConversation, "id" | "positionCursor" | "lastActivityAt">
type RowResults = Awaited<ReturnType<typeof readInboxConversations>>
type Page = Awaited<ReturnType<typeof loadInbox>>
type Window = Omit<InboxWindow, "conversations"> & { conversations: InboxConversation[] }

export type InboxListState = {
  ids: string[]
  positions: ListPosition[]
  rowIds: string[]
  unavailableIds: string[]
  startCursor: string
  endCursor: string
  hasBefore: boolean
  hasAfter: boolean
  pendingChanges: boolean
  attentionUnreadCount: number
  status: "initial" | "ready" | "loadingMore" | "refreshing"
  operation: InboxListOperation | null
  error: InboxListOperation | null
  revision: number
}

export type InboxListPorts = {
  page: (cursor?: string, beforeCursor?: string) => Promise<Page>
  window: (startCursor: string, endCursor: string) => Promise<Window>
  context: (anchor: InboxListAnchor) => Promise<Window>
  rows: (ids: string[]) => Promise<RowResults>
  capture: () => InboxListAnchor | null
  atTop: () => boolean
  interacting: () => boolean
  restore: (anchor: InboxListAnchor | null, moved: Set<string>, top: boolean) => void
  unavailable: (ids: string[]) => void
  unread: (count: number) => void
}

/** 管理一个页面查询的读取队列，查询切换后丢弃旧结果但不取消业务调用。 */
export class InboxListController {
  private state: InboxListState = {
    ids: [], positions: [], rowIds: [], unavailableIds: [], startCursor: "", endCursor: "",
    hasBefore: false, hasAfter: false, pendingChanges: false,
    attentionUnreadCount: 0, status: "initial", operation: null, error: null, revision: 0,
  }
  private listeners = new Set<() => void>()
  private queue: InboxListOperation[] = []
  private running = false
  private completion = Promise.resolve()
  private generation = 0
  private headIds: string[] = []
  private ports: InboxListPorts
  private query: InboxQuery

  /** 绑定当前页面的读取和视口适配器。 */
  constructor(ports: InboxListPorts, query: InboxQuery) { this.ports = ports; this.query = query }

  /** 返回可供 React 订阅的稳定状态快照。 */
  getSnapshot = () => this.state

  /** 订阅已提交的窗口与局部请求状态。 */
  subscribe = (listener: () => void) => {
    this.listeners.add(listener)
    return () => { this.listeners.delete(listener) }
  }

  /** 通知视图一次完整的状态变更。 */
  private publish(change: Partial<InboxListState>) {
    this.state = { ...this.state, ...change }
    this.listeners.forEach((listener) => listener())
  }

  /** 失效当前队列，已发出的读取可以自然结束。 */
  dispose() {
    this.generation++
    this.queue = []
    this.publish({ operation: null })
  }

  /** 独立详情先确认失权时立即移除该行，旧批量响应不能恢复它。 */
  removeUnavailable(id: string) {
    if (!this.state.ids.includes(id)) return
    this.generation++
    this.queue = []
    this.ports.restore(this.ports.capture(), new Set([id]), false)
    this.publish({
      ids: this.state.ids.filter((value) => value !== id),
      positions: this.state.positions.filter((position) => position.id !== id),
      unavailableIds: [...new Set([...this.state.unavailableIds, id])],
      revision: this.state.revision + 1, operation: null,
    })
    void this.request("refresh")
  }

  /** 合并重复请求并串行执行，返回本轮队列完成或过期后的 Promise。 */
  request = (operation: InboxListOperation): Promise<void> => {
    if (this.queue.includes(operation) || (this.running && this.state.operation === operation)) return this.completion
    if (operation === "poll" && (this.running || this.queue.length)) return this.completion
    this.queue.push(operation)
    if (!this.running) this.completion = this.drain()
    return this.completion
  }

  /** 轮询失败时主动重读原窗口，其余失败重试原操作。 */
  retry = () => this.request(this.state.error === "poll" ? "refresh" : this.state.error ?? "refresh")

  /** 等待补页收尾后才捕获刷新范围，失败不修改已确认的窗口。 */
  private async drain() {
    if (this.running) return
    this.running = true
    const generation = this.generation
    try {
      while (this.queue.length && generation === this.generation) {
        const operation = this.queue.shift()!
        if ((operation === "before" && !this.state.hasBefore) || (operation === "after" && !this.state.hasAfter)) continue
        if (operation === "poll" && this.state.error) continue
        this.publish({
          operation, error: null,
          status: this.state.revision === 0 ? "initial" : operation === "before" || operation === "after" ? "loadingMore" : "refreshing",
        })
        try {
          await this.execute(operation, generation)
        } catch (error) {
          if (generation !== this.generation) return
          console.warn("读取收件箱窗口失败", { query: this.query, operation, error })
          this.publish({ error: operation })
        }
        if (generation === this.generation) this.publish({ status: "ready", operation: null })
      }
    } finally {
      this.running = false
      if (this.queue.length) await this.drain()
    }
  }

  /** 按用户意图读取首页、相邻页或已加载的完整范围。 */
  private async execute(operation: InboxListOperation, generation: number) {
    const base = this.state
    const anchor = this.ports.capture()
    const pagination = operation === "before" || operation === "after"
    const latest = operation === "latest" || !base.startCursor
    // 局部窗口的顶边不等于整个筛选的顶部。
    const follow = !pagination && !base.hasBefore && this.ports.atTop() && !this.ports.interacting()
    let head: Page | undefined
    let window: Window
    if (pagination) {
      const page = await this.ports.page(operation === "after" ? base.endCursor : "", operation === "before" ? base.startCursor : "")
      window = { ...page, hasAfter: page.hasMore }
    } else {
      // 首页同时提供全量未读与窗口外新增提示，不用已加载行累加总数。
      head = await this.ports.page()
      window = latest
        ? { ...head, hasAfter: head.hasMore }
        : await this.ports.window(base.startCursor, base.endCursor)
      // 顶部新增跨过一页时重读连续扩展范围，不把首页和旧窗口直接拼接。
      if (!latest && follow && window.hasBefore && head.startCursor) window = await this.ports.window(head.startCursor, base.endCursor)
      if (!latest && !window.conversations.length) {
        if (follow) window = { ...head, hasAfter: head.hasMore }
        else if (anchor) window = await this.ports.context(anchor)
      }
    }
    if (generation !== this.generation) return
    const rowIds = [...new Set([...base.ids, ...window.conversations.map((row) => row.id)])].sort()
    const rows = await this.ports.rows(rowIds)
    if (generation !== this.generation) return
    this.commit(operation, base, window, rows, rowIds, head, follow)
  }

  /** 根据当前资格提交顺序，深处轮询只替换内容并立即移除失权或筛选外的行。 */
  private commit(operation: InboxListOperation, base: InboxListState, window: Window, rows: RowResults, rowIds: string[], head: Page | undefined, follow: boolean) {
    const matching = new Map(rows.results.filter((row) => row.availability === "matching" && row.conversation).map((row) => [row.id, row.conversation!]))
    const previous = base.ids.filter((id) => matching.has(id))
    const incoming = window.conversations.filter((row) => matching.has(row.id))
    const moved = new Set(base.positions.filter((position) => matching.has(position.id) && matching.get(position.id)!.lastActivityAt !== position.lastActivityAt).map((position) => position.id))
    const anchor = this.ports.capture()
    const latest = operation === "latest" || base.revision === 0
    follow = follow && this.ports.atTop()
    const reorder = latest || ((operation === "refresh" || follow) && !this.ports.interacting())
    let ids = previous
    if (operation === "after") ids = [...new Set([...previous, ...incoming.map((row) => row.id)])]
    else if (operation === "before") ids = [...new Set([...incoming.map((row) => row.id).filter((id) => !previous.includes(id)), ...previous])]
    else if (reorder || !previous.length) ids = incoming.map((row) => row.id)
    const candidateIds = incoming.map((row) => row.id)
    const headChanged = head !== undefined && head.conversations.map((row) => row.id).join() !== this.headIds.join()
    const pagination = operation === "before" || operation === "after"
    let pendingChanges = base.pendingChanges || moved.size > 0
    if (!pagination) {
      if (latest || (reorder && !window.hasBefore)) pendingChanges = false
      else if (reorder) pendingChanges = window.hasBefore && (pendingChanges || headChanged)
      else pendingChanges ||= headChanged || candidateIds.join() !== previous.join()
    }
    const positions = new Map(base.positions.map((position) => [position.id, position]))
    for (const row of incoming) {
      if (reorder || !positions.has(row.id)) positions.set(row.id, { id: row.id, positionCursor: row.positionCursor, lastActivityAt: row.lastActivityAt })
    }
    if (head) this.headIds = head.conversations.map((row) => row.id)
    const before = operation === "after" ? base.startCursor : window.startCursor || base.startCursor
    const after = operation === "before" ? base.endCursor : window.endCursor || base.endCursor
    this.ports.restore(anchor, reorder ? moved : new Set(), latest || (follow && reorder))
    this.publish({
      ids, rowIds, positions: ids.map((id) => positions.get(id)!),
      startCursor: latest ? window.startCursor : before, endCursor: latest ? window.endCursor : after,
      hasBefore: operation === "after" ? base.hasBefore : window.hasBefore,
      hasAfter: operation === "before" ? base.hasAfter : window.hasAfter,
      pendingChanges, attentionUnreadCount: head?.attentionUnreadCount ?? base.attentionUnreadCount,
      revision: base.revision + 1,
    })
    const unavailable = rows.results.filter((row) => row.availability === "unavailable" && base.ids.includes(row.id)).map((row) => row.id)
    if (unavailable.length) this.ports.unavailable(unavailable)
    if (head) this.ports.unread(head.attentionUnreadCount)
    if (base.revision === 0) console.info("收件箱窗口已加载", { query: this.query, conversationCount: ids.length })
  }
}

/** 规范化查询身份，内部与全部范围不携带客户筛选。 */
export function normalizeInboxListQuery(query: InboxQuery): InboxQuery {
  return {
    scope: query.scope,
    customerView: query.scope === "customer" ? query.customerView : "queue" as InboxQuery["customerView"],
    assigneeIdentityId: query.scope === "customer" && query.customerView === "coworkers" ? query.assigneeIdentityId : "",
  }
}
