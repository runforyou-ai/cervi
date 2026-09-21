/** 验证收件箱筛选的范围规则与地址参数往返。 */
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { stripTypeScriptTypes } from "node:module"
import { runInNewContext } from "node:vm"
import { test } from "node:test"

const ConversationType = {
  $zero: "",
  ConversationTypeCustomer: "customer",
  ConversationTypeDirect: "direct",
  ConversationTypeGroup: "group",
  ConversationTypeAgent: "agent",
}
const InboxScope = {
  $zero: "",
  InboxScopeAll: "all",
  InboxScopeCustomer: "customer",
  InboxScopeInternal: "internal",
}
const CustomerInboxView = {
  $zero: "",
  CustomerInboxViewQueue: "queue",
  CustomerInboxViewMine: "mine",
  CustomerInboxViewCoworkers: "coworkers",
}
const CustomerQueueFilter = {
  $zero: "",
  CustomerQueueFilterAll: "all",
  CustomerQueueFilterPublic: "public",
  CustomerQueueFilterTeam: "team",
}
const ServiceSessionStatus = {
  $zero: "",
  ServiceSessionStatusOpen: "open",
  ServiceSessionStatusClosed: "closed",
}
const InboxPartition = {
  $zero: "",
  InboxPartitionAll: "all",
  InboxPartitionPinned: "pinned",
  InboxPartitionRegular: "regular",
}
const InboxSearchRange = {
  $zero: "",
  InboxSearchRangeList: "list",
  InboxSearchRangeReadable: "readable",
  InboxSearchRangeConversation: "conversation",
}
const optionalWailsEnum = (values: Record<string, string>, value: string | null) =>
  value === null || value === values.$zero || !Object.values(values).includes(value)
    ? undefined
    : value

type InboxQuery = {
  partition?: string
  scope: string
  customerView: string
  queueFilter?: string
  queueTeamId?: string
  assigneeIdentityId: string
  channelId: string
  serviceStatus: string
  kinds: string[]
  search?: string
  searchRange?: string
}

const source = readFileSync(new URL("../src/features/inbox/inbox-query.ts", import.meta.url), "utf8")
const module: {
  normalizeInboxQuery?: (query: InboxQuery) => InboxQuery
  inboxQueryFromSearch?: (params: URLSearchParams) => InboxQuery
  writeInboxQuerySearch?: (params: URLSearchParams, query: InboxQuery) => void
  toggleInboxKinds?: (scope: string, kinds: string[], kind: string, checked: boolean) => string[]
} = {}
runInNewContext(
  stripTypeScriptTypes(source).replace(/^import\s[\s\S]*?from "[^"]+"\n/gm, "").replaceAll("export ", "") +
    "\nObject.assign(module, { normalizeInboxQuery, inboxQueryFromSearch, writeInboxQuerySearch, toggleInboxKinds })",
  { module, ConversationType, InboxScope, InboxPartition, InboxSearchRange, CustomerInboxView, CustomerQueueFilter, ServiceSessionStatus, optionalWailsEnum, URLSearchParams },
)
const { normalizeInboxQuery, inboxQueryFromSearch, writeInboxQuerySearch, toggleInboxKinds } = module as Required<typeof module>

/** 跨 VM 上下文返回的结果按值比较。 */
function plain<Value>(value: Value): Value {
  return JSON.parse(JSON.stringify(value))
}

test("范围外的客户条件和会话类型按空值规范化", () => {
  assert.deepEqual(
    plain(normalizeInboxQuery({ scope: "internal", customerView: "coworkers", assigneeIdentityId: "someone", channelId: "channel", serviceStatus: "closed", kinds: ["customer", "group"] })),
    { partition: "all", scope: "internal", customerView: "queue", queueFilter: "", queueTeamId: "", assigneeIdentityId: "", channelId: "", serviceStatus: "open", kinds: ["group"], search: "", searchRange: "list" },
  )
  assert.deepEqual(
    plain(normalizeInboxQuery({ scope: "customer", customerView: "mine", assigneeIdentityId: "someone", channelId: "channel", serviceStatus: "closed", kinds: ["group"] })),
    { partition: "all", scope: "customer", customerView: "mine", queueFilter: "", queueTeamId: "", assigneeIdentityId: "", channelId: "channel", serviceStatus: "closed", kinds: [], search: "", searchRange: "list" },
  )
})

test("待分配视图的服务状态固定为未关闭", () => {
  assert.equal(normalizeInboxQuery({ scope: "customer", customerView: "queue", queueFilter: "", queueTeamId: "", assigneeIdentityId: "", channelId: "", serviceStatus: "closed", kinds: [] }).serviceStatus, "open")
  assert.equal(inboxQueryFromSearch(new URLSearchParams("scope=customer&view=queue&status=closed")).serviceStatus, "open")
})

test("勾满当前范围全部类型等同不限类型", () => {
  assert.deepEqual(plain(toggleInboxKinds("internal", ["direct", "group"], "agent", true)), [])
  assert.deepEqual(plain(toggleInboxKinds("internal", ["direct", "group"], "direct", false)), ["group"])
  assert.deepEqual(plain(toggleInboxKinds("all", [], "group", true)), ["group"])
})

test("地址参数往返保持规范化结果，默认值不写入", () => {
  const original = new URLSearchParams("scope=customer&view=coworkers&assignee=peer&channel=web&status=closed&conversation=kept")
  const query = inboxQueryFromSearch(original)
  assert.deepEqual(plain(query), { partition: "all", scope: "customer", customerView: "coworkers", queueFilter: "", queueTeamId: "", assigneeIdentityId: "peer", channelId: "web", serviceStatus: "closed", kinds: [], search: "", searchRange: "list" })
  const written = new URLSearchParams(original)
  writeInboxQuerySearch(written, query)
  assert.equal(written.toString(), original.toString())
  // 切到内部范围时客户条件和范围外类型一起清空。
  const internal = new URLSearchParams(original)
  writeInboxQuerySearch(internal, normalizeInboxQuery({ ...query, scope: "internal", kinds: ["customer"] }))
  assert.equal(internal.toString(), "scope=internal&conversation=kept")
  // 置顶分区由列表控制器决定，不进入地址参数。
  const pinned = new URLSearchParams(original)
  writeInboxQuerySearch(pinned, normalizeInboxQuery({ ...query, partition: "pinned" }))
  assert.equal(pinned.toString(), original.toString())
  // 会话名称搜索只用于搜索模式的分页列表，不进入列表地址参数。
  const searched = new URLSearchParams(original)
  writeInboxQuerySearch(searched, normalizeInboxQuery({ ...query, search: "周报", searchRange: "readable" }))
  assert.equal(searched.toString(), original.toString())
})

test("队列筛选只在客户范围的待分配视图生效", () => {
  const team = "5f3c4a20-6c4e-4c4a-9a3a-1f2b3c4d5e6f"
  const queued = inboxQueryFromSearch(new URLSearchParams(`scope=customer&queue=${team}`))
  assert.deepEqual(plain(queued), { partition: "all", scope: "customer", customerView: "queue", queueFilter: "team", queueTeamId: team, assigneeIdentityId: "", channelId: "", serviceStatus: "open", kinds: [], search: "", searchRange: "list" })
  const written = new URLSearchParams()
  writeInboxQuerySearch(written, queued)
  assert.equal(written.toString(), `scope=customer&queue=${team}`)
  // 公共队列写入固定取值，全部队列是默认值不写入。
  const publicQueue = new URLSearchParams()
  writeInboxQuerySearch(publicQueue, normalizeInboxQuery({ ...queued, queueFilter: "public", queueTeamId: "" }))
  assert.equal(publicQueue.toString(), "scope=customer&queue=public")
  const allQueues = new URLSearchParams()
  writeInboxQuerySearch(allQueues, normalizeInboxQuery({ ...queued, queueFilter: "all", queueTeamId: "" }))
  assert.equal(allQueues.toString(), "scope=customer")
  // 切到其他视图或其他范围时队列筛选一并清空。
  const mine = new URLSearchParams()
  writeInboxQuerySearch(mine, normalizeInboxQuery({ ...queued, customerView: "mine" }))
  assert.equal(mine.toString(), "scope=customer&view=mine")
  const internal = new URLSearchParams()
  writeInboxQuerySearch(internal, normalizeInboxQuery({ ...queued, scope: "internal" }))
  assert.equal(internal.toString(), "scope=internal")
  // 指定队列缺少团队编号时按全部队列处理。
  assert.equal(plain(normalizeInboxQuery({ ...queued, queueTeamId: "" })).queueFilter, "all")
})
