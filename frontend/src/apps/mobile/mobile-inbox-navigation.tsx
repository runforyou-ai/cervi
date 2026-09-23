/** 移动端消息页签、地址查询和页签筛选面板。 */
import { useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import {
  ConversationType,
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
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import {
  chatKindOptions,
  inboxAssigneeFromParam,
  inboxAssigneeParam,
  inboxPendingKindOptions,
  inboxQueryFromSearch,
  inboxQueueFromParam,
  inboxQueueParam,
  normalizeInboxQuery,
  serviceAudienceOptions,
  toggleChatKinds,
  writeInboxQuerySearch,
} from "@/features/inbox/inbox-query"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

/** 移动端消息页签的展示顺序，页签按钮和左右滑动共用。 */
export const mobileInboxTabs = [
  { value: InboxScope.InboxScopeChat, label: "tabChat" },
  { value: InboxScope.InboxScopePending, label: "tabPending" },
  { value: InboxScope.InboxScopeAll, label: "tabAll" },
] as const

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

/** 从地址派生与服务端及桌面端一致的完整消息查询。 */
export function useMobileInboxQuery() {
  const [params, setParams] = useSearchParams()
  const query = inboxQueryFromSearch(params, mobileInboxTabs.map((tab) => tab.value))

  /** 更换筛选时替换当前列表地址并保留导航层级。 */
  function changeQuery(changes: Partial<InboxQuery>) {
    const search = new URLSearchParams()
    writeInboxQuerySearch(search, normalizeInboxQuery({ ...query, ...changes }))
    setParams(search, { replace: true })
  }
  return { query, changeQuery }
}

/** 移动端当前列表的完整筛选。 */
export type MobileInboxQuery = ReturnType<typeof useMobileInboxQuery>["query"]

/** 展示聊天、待处理与全部三个页签，聊天提示未读提醒，待处理提示本人待处理数。 */
export function MobileInboxScopes({
  scope,
  attentionUnreadCount,
  pendingCount,
  onChange,
}: {
  scope: InboxScope
  attentionUnreadCount: number
  pendingCount: number
  onChange: (query: Partial<InboxQuery>) => void
}) {
  const { t } = useTranslation("inbox")
  return (
    <nav
      aria-label={t("tabLabel")}
      className="grid shrink-0 grid-cols-3 border-b px-4"
    >
      {mobileInboxTabs.map(({ value, label }) => {
        const count = value === InboxScope.InboxScopeChat ? attentionUnreadCount : value === InboxScope.InboxScopePending ? pendingCount : 0
        return (
          <button
            key={value}
            type="button"
            aria-pressed={scope === value}
            onClick={() => onChange({ scope: value })}
            className={cn(
              "min-h-11 border-b-2 border-transparent px-2 text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
              scope === value
                ? "border-primary text-primary"
                : "text-muted-foreground",
            )}
          >
            <span className="relative">
              {t(label)}
              {count > 0 ? (
                <span
                  className="absolute -top-0.5 -right-2 size-2 rounded-full bg-destructive"
                  role="status"
                  aria-label={t(value === InboxScope.InboxScopeChat ? "chatAttentionUnread" : "pendingCount", { count })}
                />
              ) : null}
            </span>
          </button>
        )
      })}
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

/** 在底部面板中按当前页签选择筛选条件，取消时保留原筛选；尚未接入的服务对象只展示不可选。 */
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
  const { t: tMobile } = useTranslation(["mobile", "common"])
  const { identity } = useMobileWorkspace()
  const pending = query.scope === InboxScope.InboxScopePending
  const all = query.scope === InboxScope.InboxScopeAll
  const service = pending || all
  const [open, setOpen] = useState(false)
  const [pendingKind, setPendingKind] = useState(query.pendingKind)
  // 队列与负责人筛选各用单值表示，与地址参数一致。
  const [queue, setQueue] = useState(inboxQueueParam(query))
  const [assignee, setAssignee] = useState(inboxAssigneeParam(query))
  const [channel, setChannel] = useState(query.channelId)
  const [audience, setAudience] = useState(query.audience)
  const [status, setStatus] = useState(query.serviceStatus)
  const [kinds, setKinds] = useState<ConversationType[]>(query.kinds)
  const { data: channels = [] } = useResource(
    resourceKeys.inboxChannels(),
    listInboxChannels,
    { enabled: service, staleTime: 0 },
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
  const summary = service
    ? [
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
    : [
        query.kinds.length
          ? chatKindOptions
              .filter((option) => query.kinds.includes(option.kind))
              .map((option) => t(option.label))
              .join("、")
          : t("filterAllKinds"),
      ]
  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (next) {
          setPendingKind(query.pendingKind)
          setQueue(inboxQueueParam(query))
          setAssignee(inboxAssigneeParam(query))
          setChannel(query.channelId)
          setAudience(query.audience)
          setStatus(query.serviceStatus)
          setKinds(query.kinds)
        }
        setOpen(next)
        onOpenChange(next)
      }}
    >
      <SheetTrigger asChild>
        <Button
          variant="ghost"
          className="min-h-11 min-w-0 max-w-full justify-start overflow-hidden px-4 text-xs"
        >
          <span className="min-w-0 truncate">
            {tMobile("inbox.filterSummary", {
              summary: summary.filter(Boolean).join(" · ") || t("filterAll"),
            })}
          </span>
        </Button>
      </SheetTrigger>
      <SheetContent
        side="bottom"
        showCloseButton={false}
        aria-describedby={undefined}
        className="max-h-[calc(100dvh-env(safe-area-inset-top)-1rem)] gap-0 rounded-t-2xl pb-[env(safe-area-inset-bottom)]"
      >
        <SheetHeader className="flex-row items-center border-b">
          <SheetTitle className="flex-1">{t("filterLabel")}</SheetTitle>
          <SheetClose asChild>
            <Button variant="ghost" className="min-h-11">
              {tMobile("common:actions.cancel")}
            </Button>
          </SheetClose>
        </SheetHeader>
        <div className="overflow-y-auto p-4 space-y-9">
          {service ? (
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
          ) : (
            <fieldset className="space-y-2">
              <legend className="pb-2 text-sm font-medium">{t("filterKind")}</legend>
              {chatKindOptions.map((option) => (
                <label key={option.kind} className="flex min-h-11 items-center gap-3 text-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={kinds.includes(option.kind)}
                    onChange={(event) =>
                      setKinds(toggleChatKinds(kinds, option.kind, event.target.checked))
                    }
                  />
                  <span>{t(option.label)}</span>
                </label>
              ))}
            </fieldset>
          )}
          <Button
            className="min-h-11 w-full"
            onClick={() => {
              onChange(
                service
                  ? {
                      pendingKind,
                      ...inboxQueueFromParam(queue),
                      ...inboxAssigneeFromParam(assignee),
                      channelId: channel,
                      audience,
                      serviceStatus: status,
                    }
                  : { kinds },
              )
              setOpen(false)
              onOpenChange(false)
            }}
          >
            {tMobile("apply")}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
