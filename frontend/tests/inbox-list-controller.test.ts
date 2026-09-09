/** 用可控响应顺序验证分页、窗口刷新与查询代次隔离。 */
import assert from "node:assert/strict"
import { test } from "node:test"
import { InboxListController, normalizeInboxListQuery, type InboxListPorts } from "../src/features/inbox/inbox-list-controller.ts"
import type { InboxConversation, InboxQuery } from "../src/api/index.ts"

/** 构造保持服务端精度的排序位置。 */
function row(number: number): InboxConversation {
  return { id: String(number), positionCursor: `p${number}`, lastActivityAt: `2026-09-09T00:00:00.${String(1000 - number).padStart(6, "0")}Z` } as InboxConversation
}

/** 构造具有双向边界的权威窗口。 */
function windowPage(start: number, end: number) {
  return {
    conversations: Array.from({ length: Math.max(0, end - start + 1) }, (_, i) => row(start + i)),
    startCursor: `p${start}`, endCursor: `p${end}`, hasBefore: start > 1, hasAfter: end < 220,
  }
}

/** 创建可替换响应的真实控制器与读取适配器。 */
function fixture() {
  const trace: string[] = []
  const records = new Map(Array.from({ length: 220 }, (_, i) => [String(i + 1), row(i + 1)]))
  let top = true
  let interacting = false
  let restored: Parameters<InboxListPorts["restore"]> | undefined
  const unavailable: string[] = []
  const ports: InboxListPorts = {
    page: async (cursor = "", before = "") => {
      trace.push(`page:${cursor}:${before}`)
      const start = cursor ? Number(cursor.slice(1)) + 1 : before ? Math.max(1, Number(before.slice(1)) - 50) : 1
      const window = windowPage(start, before ? Number(before.slice(1)) - 1 : Math.min(start + 49, 220))
      return { ...window, hasMore: window.hasAfter, nextCursor: window.endCursor, unreadCount: 99, attentionUnreadCount: 88 }
    },
    window: async (start, end) => { trace.push(`window:${start}:${end}`); return windowPage(Number(start.slice(1)), Number(end.slice(1))) },
    context: async () => { trace.push("context"); return windowPage(81, 130) },
    rows: async (ids) => ({ results: ids.map((id) => ({ id, availability: records.has(id) ? "matching" : "unavailable", conversation: records.get(id) ?? null })) }) as Awaited<ReturnType<InboxListPorts["rows"]>>,
    capture: () => ({ id: "80", cursor: "p80", neighbors: [{ id: "80", offset: -10 }, { id: "81", offset: 58 }, { id: "79", offset: -78 }] }),
    atTop: () => top,
    interacting: () => interacting,
    restore: (...args) => { restored = args },
    unavailable: (ids) => { unavailable.push(...ids) },
    unread: () => {},
  }
  const controller = new InboxListController(ports, { scope: "all", customerView: "queue", assigneeIdentityId: "" } as InboxQuery)
  return { controller, ports, trace, records, unavailable, top: (value: boolean) => { top = value }, interact: (value: boolean) => { interacting = value }, restored: () => restored }
}

test("补页在途时合并重复触底，刷新等待完整扩展区间且不丢页", async () => {
  const f = fixture()
  const initialPage = f.ports.page
  let initial = true
  f.ports.page = (cursor, before) => {
    if (!cursor && !before && initial) { initial = false; return initialPage("p50") }
    return initialPage(cursor, before)
  }
  await f.controller.request("initial")
  f.top(false)
  await f.controller.request("after")
  const gate = Promise.withResolvers<Awaited<ReturnType<InboxListPorts["page"]>>>()
  const original = f.ports.page
  f.ports.page = (cursor, before) => cursor === "p150" ? gate.promise : original(cursor, before)
  const pending = f.controller.request("after")
  void f.controller.request("after")
  void f.controller.request("refresh")
  void f.controller.request("refresh")
  assert.equal(f.controller.getSnapshot().status, "loadingMore")
  gate.resolve(await original("p150"))
  await pending
  assert.equal(f.trace.filter((call) => call === "page:p150:").length, 1)
  assert.equal(f.trace.filter((call) => call === "window:p51:p200").length, 1)
  assert.equal(f.controller.getSnapshot().ids.length, 150)
  assert.equal(f.controller.getSnapshot().endCursor, "p200")
})

test("深处轮询暂存上浮，显式刷新后锚定原邻居且保留完整窗口", async () => {
  const f = fixture()
  await f.controller.request("initial")
  await f.controller.request("after")
  f.top(false)
  f.records.set("80", { ...row(80), lastActivityAt: "2026-09-10T00:00:00.000001Z" })
  f.ports.window = async () => ({ ...windowPage(1, 100), conversations: windowPage(1, 100).conversations.filter((row) => row.id !== "80") })
  await f.controller.request("poll")
  assert.equal(f.controller.getSnapshot().ids[79], "80")
  assert.equal(f.controller.getSnapshot().pendingChanges, true)
  assert.equal(f.restored()![1].size, 0)
  await f.controller.request("refresh")
  assert.equal(f.controller.getSnapshot().ids[79], "81")
  assert.ok(f.restored()![1].has("80"))
  assert.equal(f.restored()![2], false)
  assert.equal(f.controller.getSnapshot().ids.length, 99)
})

test("补页和刷新失败保留已确认内容与边界，重试相同范围", async () => {
  const f = fixture()
  await f.controller.request("initial")
  f.top(false)
  const base = f.controller.getSnapshot()
  const page = f.ports.page
  f.ports.page = async () => { throw new Error("offline") }
  await f.controller.request("after")
  assert.deepEqual(f.controller.getSnapshot().ids, base.ids)
  assert.equal(f.controller.getSnapshot().endCursor, base.endCursor)
  assert.equal(f.controller.getSnapshot().error, "after")
  f.ports.page = page
  await f.controller.request("after")
  assert.equal(f.controller.getSnapshot().ids.length, 100)
  const read = f.ports.window
  f.ports.window = async () => { throw new Error("offline") }
  await f.controller.request("refresh")
  assert.equal(f.controller.getSnapshot().error, "refresh")
  assert.equal(f.controller.getSnapshot().ids.length, 100)
  f.ports.window = read
  await f.controller.request("refresh")
  assert.equal(f.trace.at(-1), "window:p1:p100")
  assert.equal(f.controller.getSnapshot().error, null)
})

test("刷新在途的补页排队，旧查询结果不写入已失效的控制器", async () => {
  const f = fixture()
  await f.controller.request("initial")
  f.top(false)
  const gate = Promise.withResolvers<ReturnType<typeof windowPage>>()
  f.ports.window = () => gate.promise
  const pending = f.controller.request("refresh")
  void f.controller.request("after")
  gate.resolve(windowPage(1, 50))
  await pending
  assert.equal(f.controller.getSnapshot().endCursor, "p100")
  const old = Promise.withResolvers<ReturnType<typeof windowPage>>()
  f.ports.window = () => old.promise
  const obsolete = f.controller.request("refresh")
  f.controller.dispose()
  old.resolve(windowPage(1, 200))
  await obsolete
  assert.equal(f.controller.getSnapshot().endCursor, "p100")
})

test("空区间恢复原邻域，失权立即移除并清理详情", async () => {
  const f = fixture()
  await f.controller.request("initial")
  await f.controller.request("after")
  f.top(false)
  f.records.delete("80")
  f.ports.window = async () => ({ ...windowPage(1, 100), conversations: [] })
  await f.controller.request("refresh")
  assert.ok(f.trace.includes("context"))
  assert.deepEqual(f.unavailable, ["80"])
  assert.equal(f.controller.getSnapshot().ids[0], "81")
  assert.equal(f.controller.getSnapshot().hasBefore, true)
})

test("读取期间用户离开顶部或操作菜单时不自动回顶重排", async () => {
  const f = fixture()
  await f.controller.request("initial")
  const read = f.ports.rows
  f.ports.rows = async (ids) => { f.top(false); return read(ids) }
  await f.controller.request("poll")
  assert.equal(f.restored()![2], false)
  f.interact(true)
  await f.controller.request("refresh")
  assert.equal(f.restored()![2], false)
  f.interact(false)
  await f.controller.request("latest")
  assert.equal(f.restored()![2], true)
  assert.equal(f.controller.getSnapshot().pendingChanges, false)
  assert.equal(f.controller.getSnapshot().attentionUnreadCount, 88)
})

test("无关客户参数不产生不同内部查询身份", () => {
  assert.deepEqual(normalizeInboxListQuery({ scope: "internal", customerView: "coworkers", assigneeIdentityId: "someone" } as InboxQuery), { scope: "internal", customerView: "queue", assigneeIdentityId: "" })
})

test("独立详情先失权时仅移除该行，并阻止旧窗口读取恢复它", async () => {
  const f = fixture()
  await f.controller.request("initial")
  f.top(false)
  const gate = Promise.withResolvers<ReturnType<typeof windowPage>>()
  const read = f.ports.window
  f.ports.window = () => gate.promise
  const pending = f.controller.request("refresh")
  f.records.delete("20")
  f.controller.removeUnavailable("20")
  assert.equal(f.controller.getSnapshot().ids.length, 49)
  assert.ok(!f.controller.getSnapshot().ids.includes("20"))
  f.ports.window = read
  gate.resolve(windowPage(1, 50))
  await pending
  assert.ok(!f.controller.getSnapshot().ids.includes("20"))
  assert.equal(f.controller.getSnapshot().endCursor, "p50")
  assert.deepEqual(f.controller.getSnapshot().unavailableIds, ["20"])
})

test("普通补页不产生待更新提示，批量资格读取失败不接纳半个页面", async () => {
  const f = fixture()
  await f.controller.request("initial")
  f.top(false)
  await f.controller.request("after")
  assert.equal(f.controller.getSnapshot().pendingChanges, false)
  const rows = f.ports.rows
  f.ports.rows = async () => { throw new Error("rows offline") }
  await f.controller.request("after")
  assert.equal(f.controller.getSnapshot().ids.length, 100)
  assert.equal(f.controller.getSnapshot().endCursor, "p100")
  f.ports.rows = rows
  await f.controller.request("after")
  assert.equal(f.controller.getSnapshot().ids.length, 150)
})

test("顶部新增超过一页时读取连续扩展范围，不遗漏首页与旧区间之间的行", async () => {
  const f = fixture()
  await f.controller.request("initial")
  const original = f.ports.page
  f.ports.page = async () => ({ ...await original(), startCursor: "new-start" })
  f.ports.window = async (start, end) => {
    f.trace.push(`window:${start}:${end}`)
    return { ...windowPage(1, start === "new-start" ? 120 : 50), hasBefore: start !== "new-start" }
  }
  await f.controller.request("poll")
  assert.deepEqual(f.trace.filter((call) => call.startsWith("window:")), ["window:p1:p50", "window:new-start:p50"])
  assert.equal(f.controller.getSnapshot().ids.length, 120)
})

test("轮询失败后通过重试恢复原窗口，并继续后续轮询", async () => {
  const f = fixture()
  await f.controller.request("initial")
  await f.controller.request("after")
  f.top(false)
  const read = f.ports.window
  f.ports.window = async () => { throw new Error("poll offline") }
  await f.controller.request("poll")
  assert.equal(f.controller.getSnapshot().error, "poll")
  assert.equal(f.controller.getSnapshot().ids.length, 100)
  f.ports.window = read
  await f.controller.retry()
  assert.equal(f.controller.getSnapshot().error, null)
  assert.equal(f.controller.getSnapshot().endCursor, "p100")
  assert.equal(f.restored()![2], false)
  const calls = f.trace.length
  await f.controller.request("poll")
  assert.ok(f.trace.length > calls)
  assert.equal(f.controller.getSnapshot().ids.length, 100)
})

test("排队与合并请求返回的 Promise 等待整轮实际读取完成", async () => {
  const f = fixture()
  await f.controller.request("initial")
  f.top(false)
  const gate = Promise.withResolvers<ReturnType<typeof windowPage>>()
  f.ports.window = () => gate.promise
  const refreshing = f.controller.request("refresh")
  const more = f.controller.request("after")
  const duplicate = f.controller.request("after")
  let completed = false
  void more.then(() => { completed = true })
  await Promise.resolve()
  assert.equal(completed, false)
  gate.resolve(windowPage(1, 50))
  await Promise.all([refreshing, more, duplicate])
  assert.equal(completed, true)
  assert.equal(f.controller.getSnapshot().ids.length, 100)
})
