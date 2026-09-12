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
const ServiceSessionStatus = {
  $zero: "",
  ServiceSessionStatusOpen: "open",
  ServiceSessionStatusClosed: "closed",
}
const optionalWailsEnum = (values: Record<string, string>, value: string | null) =>
  value === null || value === values.$zero || !Object.values(values).includes(value)
    ? undefined
    : value

type InboxQuery = {
  scope: string
  customerView: string
  assigneeIdentityId: string
  channelId: string
  serviceStatus: string
  kinds: string[]
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
  { module, ConversationType, InboxScope, CustomerInboxView, ServiceSessionStatus, optionalWailsEnum, URLSearchParams },
)
const { normalizeInboxQuery, inboxQueryFromSearch, writeInboxQuerySearch, toggleInboxKinds } = module as Required<typeof module>

/** 跨 VM 上下文返回的结果按值比较。 */
function plain<Value>(value: Value): Value {
  return JSON.parse(JSON.stringify(value))
}

test("范围外的客户条件和会话类型按空值规范化", () => {
  assert.deepEqual(
    plain(normalizeInboxQuery({ scope: "internal", customerView: "coworkers", assigneeIdentityId: "someone", channelId: "channel", serviceStatus: "closed", kinds: ["customer", "group"] })),
    { scope: "internal", customerView: "queue", assigneeIdentityId: "", channelId: "", serviceStatus: "open", kinds: ["group"] },
  )
  assert.deepEqual(
    plain(normalizeInboxQuery({ scope: "customer", customerView: "mine", assigneeIdentityId: "someone", channelId: "channel", serviceStatus: "closed", kinds: ["group"] })),
    { scope: "customer", customerView: "mine", assigneeIdentityId: "", channelId: "channel", serviceStatus: "closed", kinds: [] },
  )
})

test("勾满当前范围全部类型等同不限类型", () => {
  assert.deepEqual(plain(toggleInboxKinds("internal", ["direct", "group"], "agent", true)), [])
  assert.deepEqual(plain(toggleInboxKinds("internal", ["direct", "group"], "direct", false)), ["group"])
  assert.deepEqual(plain(toggleInboxKinds("all", [], "group", true)), ["group"])
})

test("地址参数往返保持规范化结果，默认值不写入", () => {
  const original = new URLSearchParams("scope=customer&view=coworkers&assignee=peer&channel=web&status=closed&conversation=kept")
  const query = inboxQueryFromSearch(original)
  assert.deepEqual(plain(query), { scope: "customer", customerView: "coworkers", assigneeIdentityId: "peer", channelId: "web", serviceStatus: "closed", kinds: [] })
  const written = new URLSearchParams(original)
  writeInboxQuerySearch(written, query)
  assert.equal(written.toString(), original.toString())
  // 切到内部范围时客户条件和范围外类型一起清空。
  const internal = new URLSearchParams(original)
  writeInboxQuerySearch(internal, normalizeInboxQuery({ ...query, scope: "internal", kinds: ["customer"] }))
  assert.equal(internal.toString(), "scope=internal&conversation=kept")
})
