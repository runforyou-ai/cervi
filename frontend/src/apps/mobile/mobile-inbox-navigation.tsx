/** 移动端收件箱页签、地址查询和服务会话筛选面板。 */
import { useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import {
  CustomerQueueFilter,
  InboxAssigneeFilter,
  InboxPendingKind,
  InboxScope,
  OrganizationIdentityType,
  ServiceAudience,
  ServiceSessionStatus,
  listCustomerServiceAssignees,
  listInboxChannels,
  listServiceQueueTeams,
  type InboxQuery,
} from "@/api"
import { MobileFilterSheet } from "@/apps/mobile/mobile-filter-sheet"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { CountBadge } from "@/components/count-badge"
import { Button } from "@/components/ui/button"
import {
  inboxAssigneeFromParam,
  inboxAssigneeParam,
  inboxPendingKindOptions,
  inboxQueryFromSearch,
  inboxQueueFromParam,
  inboxQueueParam,
  inboxTabs,
  normalizeInboxQuery,
  serviceAudienceOptions,
  writeInboxQuerySearch,
} from "@/features/inbox/inbox-query"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

const serviceStatuses = [
  {
    value: ServiceSessionStatus.ServiceSessionStatusOpen,
    label: "filterServiceStatusOpen",
  },
  {
    value: ServiceSessionStatus.ServiceSessionStatusClosed,
    label: "filterServiceStatusClosed",
  },
] as const

const selectClassName = "h-11 w-full rounded-md border bg-background px-3 text-sm"

/** 从地址派生与服务端及桌面端一致的收件箱查询。 */
export function useMobileInboxQuery() {
  const [params, setParams] = useSearchParams()
  const query = inboxQueryFromSearch(params)

  /** 更换筛选时替换当前列表地址并保留导航层级。 */
  function changeQuery(changes: Partial<InboxQuery>) {
    const search = new URLSearchParams()
    writeInboxQuerySearch(search, normalizeInboxQuery({ ...query, ...changes }))
    setParams(search, { replace: true })
  }
  return { query, changeQuery }
}

/** 移动端收件箱当前列表的完整筛选。 */
export type MobileInboxQuery = ReturnType<typeof useMobileInboxQuery>["query"]

/** 展示待处理与全部两个页签，待处理以中性徽标显示本人待处理数。 */
export function MobileInboxScopes({
  scope,
  pendingCount,
  onChange,
}: {
  scope: InboxScope
  pendingCount: number
  onChange: (query: Partial<InboxQuery>) => void
}) {
  const { t } = useTranslation("inbox")
  return (
    <nav
      aria-label={t("tabLabel")}
      className="grid shrink-0 grid-cols-2 border-b px-4"
    >
      {inboxTabs.map(({ value, label }) => (
        <button
          key={value}
          type="button"
          aria-pressed={scope === value}
          onClick={() => onChange({ scope: value })}
          className={cn(
            "flex min-h-11 items-center justify-center gap-1.5 border-b-2 border-transparent px-2 text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
            scope === value
              ? "border-primary text-primary"
              : "text-muted-foreground",
          )}
        >
          {t(label)}
          {value === InboxScope.InboxScopePending && pendingCount > 0 ? (
            <CountBadge
              count={pendingCount}
              tone="neutral"
              label={t("pendingCount", { count: pendingCount })}
            />
          ) : null}
        </button>
      ))}
    </nav>
  )
}

/** 面板中带标签的一项筛选。 */
function MobileFilterField({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <label className="block text-sm font-medium" htmlFor={id}>
        {label}
      </label>
      {children}
    </div>
  )
}

/** 在底部面板中按当前页签选择服务会话筛选条件；尚未接入的服务对象只展示不可选。 */
export function MobileInboxFilter({
  query,
  onChange,
  onOpenChange,
}: {
  query: MobileInboxQuery
  onChange: (query: Partial<InboxQuery>) => void
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation("inbox")
  const { t: tMobile } = useTranslation("mobile")
  const { identity } = useMobileWorkspace()
  const pending = query.scope === InboxScope.InboxScopePending
  const all = query.scope === InboxScope.InboxScopeAll
  const [pendingKind, setPendingKind] = useState(query.pendingKind)
  // 队列与负责人筛选各用单值表示，与地址参数一致。
  const [queue, setQueue] = useState(inboxQueueParam(query))
  const [assignee, setAssignee] = useState(inboxAssigneeParam(query))
  const [channel, setChannel] = useState(query.channelId)
  const [audience, setAudience] = useState(query.audience)
  const [status, setStatus] = useState(query.serviceStatus)
  const { data: channels = [] } = useResource(
    resourceKeys.inboxChannels(),
    listInboxChannels,
    { staleTime: 0 },
  )
  const { data: queueTeams = [] } = useResource(
    resourceKeys.serviceQueueTeams(),
    listServiceQueueTeams,
    { enabled: pending, staleTime: 0 },
  )
  const { data: assignees = [] } = useResource(
    resourceKeys.customerServiceAssignees(),
    listCustomerServiceAssignees,
    { enabled: all, staleTime: 0 },
  )
  const coworkers = assignees.filter((item) => item.identityId !== identity.user.identityId)
  const assigneeLabel = (value: string) =>
    value === identity.user.identityId
      ? t("filterAssigneeMe")
      : value === InboxAssigneeFilter.InboxAssigneeFilterUnassigned
        ? t("filterAssigneeUnassigned")
        : (coworkers.find((item) => item.identityId === value)?.displayName ?? tMobile("inbox.selectedAssignee"))
  const summary = [
    pending ? t(inboxPendingKindOptions.find((item) => item.value === query.pendingKind)?.label ?? "filterAll") : "",
    pending && query.queueFilter === CustomerQueueFilter.CustomerQueueFilterPublic
      ? t("queueFilterPublicQueue")
      : pending ? (queueTeams.find((item) => item.id === query.queueTeamId)?.name ?? "") : "",
    all && inboxAssigneeParam(query) ? assigneeLabel(inboxAssigneeParam(query)) : "",
    channels.find((item) => item.id === query.channelId)?.name ?? "",
    query.audience ? t(serviceAudienceOptions.find((item) => item.value === query.audience)?.label ?? "filterAll") : "",
    query.serviceStatus === ServiceSessionStatus.ServiceSessionStatusClosed
      ? t("filterServiceStatusClosed")
      : "",
  ]
  return (
    <MobileFilterSheet
      summary={summary.filter(Boolean).join(" · ")}
      onOpenChange={onOpenChange}
      onOpen={() => {
        setPendingKind(query.pendingKind)
        setQueue(inboxQueueParam(query))
        setAssignee(inboxAssigneeParam(query))
        setChannel(query.channelId)
        setAudience(query.audience)
        setStatus(query.serviceStatus)
      }}
      onApply={() =>
        onChange({
          pendingKind,
          ...inboxQueueFromParam(queue),
          ...inboxAssigneeFromParam(assignee),
          channelId: channel,
          audience,
          serviceStatus: status,
        })
      }
    >
      <div className="space-y-4">
        {pending ? (
          <div
            role="group"
            aria-label={t("filterPendingKind")}
            className="grid grid-cols-2 gap-2"
          >
            {[{ value: InboxPendingKind.$zero, label: "filterAll" } as const, ...inboxPendingKindOptions].map((item) => (
              <Button
                key={item.value}
                variant={pendingKind === item.value ? "default" : "outline"}
                className="min-h-11"
                aria-pressed={pendingKind === item.value}
                onClick={() => setPendingKind(item.value)}
              >
                {t(item.label)}
              </Button>
            ))}
          </div>
        ) : (
          <div role="group" aria-label={t("filterServiceStatus")} className="grid grid-cols-2 gap-2">
            {serviceStatuses.map((item) => (
              <Button
                key={item.value}
                variant={status === item.value ? "default" : "outline"}
                className="min-h-11"
                aria-pressed={status === item.value}
                onClick={() => setStatus(item.value)}
              >
                {t(item.label)}
              </Button>
            ))}
          </div>
        )}
        {pending && pendingKind === InboxPendingKind.InboxPendingKindQueue ? (
          <MobileFilterField id="mobile-inbox-queue" label={t("filterQueue")}>
            <select
              id="mobile-inbox-queue"
              className={selectClassName}
              value={queue}
              onChange={(event) => setQueue(event.target.value)}
            >
              <option value="">{t("queueFilterAllQueues")}</option>
              <option value={CustomerQueueFilter.CustomerQueueFilterPublic}>
                {t("queueFilterPublicQueue")}
              </option>
              {queueTeams.filter((item) => item.mine).map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </MobileFilterField>
        ) : null}
        {all ? (
          <MobileFilterField id="mobile-inbox-assignee" label={t("filterAssignee")}>
            <select
              id="mobile-inbox-assignee"
              className={selectClassName}
              value={assignee}
              onChange={(event) => setAssignee(event.target.value)}
            >
              <option value="">{t("filterAll")}</option>
              <option value={identity.user.identityId}>{t("filterAssigneeMe")}</option>
              <option value={InboxAssigneeFilter.InboxAssigneeFilterUnassigned}>{t("filterAssigneeUnassigned")}</option>
              {assignee && ![identity.user.identityId, InboxAssigneeFilter.InboxAssigneeFilterUnassigned as string].includes(assignee) &&
              !coworkers.some((item) => item.identityId === assignee) ? (
                <option value={assignee}>{tMobile("inbox.selectedAssignee")}</option>
              ) : null}
              {coworkers.map((item) => (
                <option key={item.identityId} value={item.identityId}>
                  {item.type === OrganizationIdentityType.OrganizationIdentityTypeAgent
                    ? t("filterAssigneeAgent", { name: item.displayName })
                    : item.displayName}
                </option>
              ))}
            </select>
          </MobileFilterField>
        ) : null}
        <MobileFilterField id="mobile-inbox-source" label={t("filterSource")}>
          <select
            id="mobile-inbox-source"
            className={selectClassName}
            value={channel}
            onChange={(event) => setChannel(event.target.value)}
          >
            <option value="">{t("filterAll")}</option>
            {channels.map((item) => (
              <option key={item.id} value={item.id}>
                {item.enabled
                  ? item.name
                  : `${item.name}（${t("filterChannelDisabled")}）`}
              </option>
            ))}
          </select>
        </MobileFilterField>
        <MobileFilterField id="mobile-inbox-audience" label={t("filterAudience")}>
          <select
            id="mobile-inbox-audience"
            className={selectClassName}
            value={audience}
            onChange={(event) => setAudience(event.target.value as ServiceAudience)}
          >
            <option value="">{t("filterAll")}</option>
            {serviceAudienceOptions.map((item) => (
              <option key={item.value} value={item.value} disabled={!item.available}>
                {t(item.label)}
              </option>
            ))}
          </select>
        </MobileFilterField>
      </div>
    </MobileFilterSheet>
  )
}
