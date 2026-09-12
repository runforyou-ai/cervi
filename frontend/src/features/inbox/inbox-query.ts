/** 收件箱列表筛选的范围规则与地址参数读写。 */
import {
  ConversationType,
  CustomerInboxView,
  InboxScope,
  ServiceSessionStatus,
  type InboxQuery,
} from "@/api"
import { optionalWailsEnum } from "@/lib/wails-enum"

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

/** 按当前范围规范化筛选，范围外条件取默认值。 */
export function normalizeInboxQuery(query: InboxQuery): NormalizedInboxQuery {
  const customer = query.scope === InboxScope.InboxScopeCustomer
  const customerView = customer
    ? query.customerView
    : CustomerInboxView.CustomerInboxViewQueue
  return {
    scope: query.scope,
    customerView,
    assigneeIdentityId:
      customerView === CustomerInboxView.CustomerInboxViewCoworkers
        ? query.assigneeIdentityId
        : "",
    channelId: customer ? query.channelId : "",
    serviceStatus:
      customer &&
      query.serviceStatus === ServiceSessionStatus.ServiceSessionStatusClosed
        ? ServiceSessionStatus.ServiceSessionStatusClosed
        : ServiceSessionStatus.ServiceSessionStatusOpen,
    kinds: normalizeInboxKinds(query.scope, query.kinds ?? []),
  }
}

/** 从地址参数解析当前列表筛选。 */
export function inboxQueryFromSearch(params: URLSearchParams): NormalizedInboxQuery {
  return normalizeInboxQuery({
    scope:
      optionalWailsEnum(InboxScope, params.get("scope")) ??
      InboxScope.InboxScopeAll,
    customerView:
      optionalWailsEnum(CustomerInboxView, params.get("view")) ??
      CustomerInboxView.CustomerInboxViewQueue,
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
