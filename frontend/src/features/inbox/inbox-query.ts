/** 收件箱列表筛选的范围规则与地址参数读写。 */
import {
  ConversationType,
  CustomerInboxView,
  CustomerQueueFilter,
  InboxPartition,
  InboxScope,
  InboxSearchRange,
  ServiceSessionStatus,
  type InboxQuery,
} from "@/api"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 消息范围页签的展示顺序，桌面端范围页签、移动端页签和左右滑动共用。 */
export const inboxScopes = [
  { value: InboxScope.InboxScopeAll, label: "scopeAll" },
  { value: InboxScope.InboxScopeCustomer, label: "scopeCustomer" },
  { value: InboxScope.InboxScopeInternal, label: "scopeInternal" },
] as const

/** 会话类型筛选项，顺序同时决定地址参数和游标的规范化顺序。 */
export const inboxKindOptions = [
  { kind: ConversationType.ConversationTypeCustomer, label: "filterKindCustomer" },
  { kind: ConversationType.ConversationTypeDirect, label: "filterKindDirect" },
  { kind: ConversationType.ConversationTypeGroup, label: "filterKindGroup" },
  { kind: ConversationType.ConversationTypeAgent, label: "filterKindAgent" },
] as const

/** 返回当前范围可筛选的会话类型，与服务端的范围校验一致。 */
export function inboxKindOptionsForScope(scope: InboxScope) {
  if (scope === InboxScope.InboxScopeCustomer) {
    return inboxKindOptions.filter(
      (option) => option.kind === ConversationType.ConversationTypeCustomer,
    )
  }
  if (scope === InboxScope.InboxScopeInternal) {
    return inboxKindOptions.filter(
      (option) => option.kind !== ConversationType.ConversationTypeCustomer,
    )
  }
  return inboxKindOptions
}

/** 按当前范围规范化会话类型筛选，丢弃范围外类型，覆盖全部类型收敛为空选择。 */
export function normalizeInboxKinds(
  scope: InboxScope,
  kinds: readonly ConversationType[],
): ConversationType[] {
  const available = inboxKindOptionsForScope(scope).map((option) => option.kind)
  const selected = available.filter((kind) => kinds.includes(kind))
  return selected.length === available.length ? [] : selected
}

/** 切换一个会话类型的选中状态。 */
export function toggleInboxKinds(
  scope: InboxScope,
  kinds: readonly ConversationType[],
  kind: ConversationType,
  checked: boolean,
): ConversationType[] {
  return normalizeInboxKinds(
    scope,
    checked ? [...kinds, kind] : kinds.filter((item) => item !== kind),
  )
}

/** 已按范围规范化的列表筛选，会话类型一律为数组。 */
export type NormalizedInboxQuery = InboxQuery & { kinds: ConversationType[] }

/** 规范化前的列表筛选，未指定分区时按完整活动序读取，未指定搜索词时不按名称搜索。 */
export type InboxQueryInput = Omit<InboxQuery, "partition" | "search" | "searchRange"> & {
  partition?: InboxPartition
  search?: string
  searchRange?: InboxSearchRange
}

/** 规范化「待分配」视图的队列筛选，指定队列缺少团队编号时按全部队列处理。 */
function normalizeQueueFilter(filter: CustomerQueueFilter, teamId: string) {
  if (filter === CustomerQueueFilter.CustomerQueueFilterTeam && teamId) {
    return { queueFilter: filter, queueTeamId: teamId }
  }
  if (filter === CustomerQueueFilter.CustomerQueueFilterPublic) {
    return { queueFilter: filter, queueTeamId: "" }
  }
  return { queueFilter: CustomerQueueFilter.CustomerQueueFilterAll, queueTeamId: "" }
}

/** 把队列筛选编码为单值：空为全部队列，public 为公共队列，其余为团队编号。 */
export function inboxQueueParam(query: Pick<InboxQuery, "queueFilter" | "queueTeamId">) {
  if (query.queueFilter === CustomerQueueFilter.CustomerQueueFilterPublic) {
    return CustomerQueueFilter.CustomerQueueFilterPublic
  }
  return query.queueFilter === CustomerQueueFilter.CustomerQueueFilterTeam
    ? query.queueTeamId
    : ""
}

/** 把队列单值解码为队列筛选和团队编号。 */
export function inboxQueueFromParam(value: string) {
  if (value === CustomerQueueFilter.CustomerQueueFilterPublic) {
    return { queueFilter: CustomerQueueFilter.CustomerQueueFilterPublic, queueTeamId: "" }
  }
  return value
    ? { queueFilter: CustomerQueueFilter.CustomerQueueFilterTeam, queueTeamId: value }
    : { queueFilter: CustomerQueueFilter.CustomerQueueFilterAll, queueTeamId: "" }
}

/** 按当前范围规范化筛选，范围外条件取默认值。 */
export function normalizeInboxQuery(query: InboxQueryInput): NormalizedInboxQuery {
  const customer = query.scope === InboxScope.InboxScopeCustomer
  const customerView = customer
    ? query.customerView
    : CustomerInboxView.CustomerInboxViewQueue
  const queue =
    customer && customerView === CustomerInboxView.CustomerInboxViewQueue
      ? normalizeQueueFilter(query.queueFilter, query.queueTeamId)
      : { queueFilter: CustomerQueueFilter.$zero, queueTeamId: "" }
  return {
    // 分区由启用置顶的调用方显式指定，其余调用方继续读取完整活动序。
    partition: query.partition ?? InboxPartition.InboxPartitionAll,
    scope: query.scope,
    customerView,
    ...queue,
    assigneeIdentityId:
      customerView === CustomerInboxView.CustomerInboxViewCoworkers
        ? query.assigneeIdentityId
        : "",
    channelId: customer ? query.channelId : "",
    // 「待分配」只列出等待承接的会话，服务状态固定为未关闭。
    serviceStatus:
      customer &&
      customerView !== CustomerInboxView.CustomerInboxViewQueue &&
      query.serviceStatus === ServiceSessionStatus.ServiceSessionStatusClosed
        ? ServiceSessionStatus.ServiceSessionStatusClosed
        : ServiceSessionStatus.ServiceSessionStatusOpen,
    kinds: normalizeInboxKinds(query.scope, query.kinds ?? []),
    search: query.search ?? "",
    searchRange: query.searchRange ?? InboxSearchRange.InboxSearchRangeList,
  }
}

/** 不带列表筛选的可读范围查询，用于全局搜索和未进入消息页时的读取。 */
export const readableInboxQuery = normalizeInboxQuery({
  scope: InboxScope.InboxScopeAll,
  customerView: CustomerInboxView.CustomerInboxViewQueue,
  queueFilter: CustomerQueueFilter.CustomerQueueFilterAll,
  queueTeamId: "",
  assigneeIdentityId: "",
  channelId: "",
  serviceStatus: ServiceSessionStatus.ServiceSessionStatusOpen,
  kinds: [],
})

/** 从地址参数解析当前列表筛选。 */
export function inboxQueryFromSearch(params: URLSearchParams): NormalizedInboxQuery {
  return normalizeInboxQuery({
    scope:
      optionalWailsEnum(InboxScope, params.get("scope")) ??
      InboxScope.InboxScopeAll,
    customerView:
      optionalWailsEnum(CustomerInboxView, params.get("view")) ??
      CustomerInboxView.CustomerInboxViewQueue,
    ...inboxQueueFromParam(params.get("queue") ?? ""),
    assigneeIdentityId: params.get("assignee") ?? "",
    channelId: params.get("channel") ?? "",
    serviceStatus:
      params.get("status") === ServiceSessionStatus.ServiceSessionStatusClosed
        ? ServiceSessionStatus.ServiceSessionStatusClosed
        : ServiceSessionStatus.ServiceSessionStatusOpen,
    kinds: (params.get("kinds") ?? "").split(",") as ConversationType[],
  })
}

/** 把已规范化的筛选写回地址参数，取默认值的条件不写入。 */
export function writeInboxQuerySearch(
  params: URLSearchParams,
  query: NormalizedInboxQuery,
) {
  const write = (name: string, value: string) =>
    value ? params.set(name, value) : params.delete(name)
  write("scope", query.scope === InboxScope.InboxScopeAll ? "" : query.scope)
  write(
    "view",
    query.customerView === CustomerInboxView.CustomerInboxViewQueue
      ? ""
      : query.customerView,
  )
  write("queue", inboxQueueParam(query))
  write("assignee", query.assigneeIdentityId)
  write("channel", query.channelId)
  write(
    "status",
    query.serviceStatus === ServiceSessionStatus.ServiceSessionStatusClosed
      ? query.serviceStatus
      : "",
  )
  write("kinds", query.kinds.join(","))
}
