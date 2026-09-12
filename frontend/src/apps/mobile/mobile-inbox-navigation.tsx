/** 移动端消息分类、地址查询和范围筛选面板。 */
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import {
  ConversationType,
  CustomerInboxView,
  InboxScope,
  ServiceSessionStatus,
  listCustomerServiceAssignees,
  listInboxChannels,
  type LoadInboxQuery,
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
  inboxKindOptionsForScope,
  inboxQueryFromSearch,
  normalizeInboxQuery,
  toggleInboxKinds,
  writeInboxQuerySearch,
} from "@/features/inbox/inbox-query"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

const scopes = [
  { value: InboxScope.InboxScopeAll, label: "scopeAll" },
  { value: InboxScope.InboxScopeCustomer, label: "scopeCustomer" },
  { value: InboxScope.InboxScopeInternal, label: "scopeInternal" },
] as const

const customerViews = [
  {
    value: CustomerInboxView.CustomerInboxViewQueue,
    label: "queueFilterQueue",
  },
  { value: CustomerInboxView.CustomerInboxViewMine, label: "queueFilterMine" },
  {
    value: CustomerInboxView.CustomerInboxViewCoworkers,
    label: "queueFilterColleague",
  },
] as const

/** 从地址派生与服务端及桌面端一致的完整消息查询。 */
export function useMobileInboxQuery() {
  const [params, setParams] = useSearchParams()
  const query = inboxQueryFromSearch(params)

  /** 更换筛选时替换当前列表地址并保留导航层级。 */
  function changeQuery(changes: LoadInboxQuery) {
    const search = new URLSearchParams()
    writeInboxQuerySearch(search, normalizeInboxQuery({ ...query, ...changes }))
    setParams(search, { replace: true })
  }
  return { query, changeQuery }
}

/** 移动端当前列表的完整筛选。 */
export type MobileInboxQuery = ReturnType<typeof useMobileInboxQuery>["query"]

/** 展示三个业务范围，内部同时包含单聊和群聊。 */
export function MobileInboxScopes({
  scope,
  onChange,
}: {
  scope: InboxScope
  onChange: (query: LoadInboxQuery) => void
}) {
  const { t } = useTranslation("inbox")
  return (
    <nav
      aria-label={t("scopeRailLabel")}
      className="grid shrink-0 grid-cols-3 border-b px-4"
    >
      {scopes.map(({ value, label }) => (
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
          {t(label)}
        </button>
      ))}
    </nav>
  )
}

/** 按需加载可选同事并通过原生选择控件切换。 */
function MobileCustomerAssignee({
  value,
  onChange,
}: {
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation(["mobile", "common"])
  const { identity } = useMobileWorkspace()
  const { data, loading, error, refresh } = useResource(
    resourceKeys.customerServiceAssignees(),
    listCustomerServiceAssignees,
    { staleTime: 0 },
  )
  const coworkers = (data ?? []).filter(
    (item) => item.identityId !== identity.user.identityId,
  )
  return (
    <div className="space-y-2">
      <label
        className="block text-sm font-medium"
        htmlFor="mobile-customer-assignee"
      >
        {t("inbox.assignee")}
      </label>
      <select
        id="mobile-customer-assignee"
        className="h-11 w-full rounded-md border bg-background px-3 text-sm"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{t("inbox.allCoworkers")}</option>
        {value && !coworkers.some((item) => item.identityId === value) ? (
          <option value={value}>{t("inbox.selectedAssignee")}</option>
        ) : null}
        {coworkers.map((item) => (
          <option key={item.identityId} value={item.identityId}>
            {item.displayName}
          </option>
        ))}
      </select>
      {loading ? (
        <p className="text-xs text-muted-foreground">{t("common:status.loading")}</p>
      ) : null}
      {error ? (
        <Button
          variant="outline"
          className="min-h-11"
          onClick={() => void refresh()}
        >
          {t("inbox.assigneesRetry")}
        </Button>
      ) : null}
    </div>
  )
}

/** 在底部面板中按当前范围选择筛选条件，取消时保留原筛选。 */
export function MobileInboxFilter({
  query,
  onChange,
  onOpenChange,
}: {
  query: MobileInboxQuery
  onChange: (query: LoadInboxQuery) => void
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation("inbox")
  const { t: tMobile } = useTranslation(["mobile", "common"])
  const customer = query.scope === InboxScope.InboxScopeCustomer
  const [open, setOpen] = useState(false)
  const [view, setView] = useState(query.customerView)
  const [assignee, setAssignee] = useState(query.assigneeIdentityId)
  const [channel, setChannel] = useState(query.channelId)
  const [status, setStatus] = useState(query.serviceStatus)
  const [kinds, setKinds] = useState<ConversationType[]>(query.kinds)
  const { data } = useResource(
    resourceKeys.customerServiceAssignees(),
    listCustomerServiceAssignees,
    { enabled: Boolean(query.assigneeIdentityId), staleTime: 0 },
  )
  const { data: channels = [] } = useResource(
    resourceKeys.inboxChannels(),
    listInboxChannels,
    { enabled: customer, staleTime: 0 },
  )
  const selected = data?.find(
    (item) => item.identityId === query.assigneeIdentityId,
  )
  const options = inboxKindOptionsForScope(query.scope)
  const viewLabel = customerViews.find(
    (item) => item.value === query.customerView,
  )!.label
  const summary = customer
    ? [
        t(viewLabel),
        query.assigneeIdentityId
          ? (selected?.displayName ?? tMobile("inbox.selectedAssignee"))
          : "",
        channels.find((item) => item.id === query.channelId)?.name ?? "",
        query.serviceStatus === ServiceSessionStatus.ServiceSessionStatusClosed
          ? t("filterServiceStatusClosed")
          : "",
      ]
    : [
        query.kinds.length
          ? options
              .filter((option) => query.kinds.includes(option.kind))
              .map((option) => t(option.label))
              .join("、")
          : t("filterKind"),
      ]
  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (next) {
          setView(query.customerView)
          setAssignee(query.assigneeIdentityId)
          setChannel(query.channelId)
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
            {tMobile("inbox.filter")}
            {"："}
            {summary.filter(Boolean).join(" · ")}
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
          {customer ? (
            <div className="space-y-4">
              <div
                role="group"
                aria-label={t("queueFilterLabel")}
                className="grid grid-cols-2 gap-2"
              >
                {customerViews.map((item) => (
                  <Button
                    key={item.value}
                    variant={view === item.value ? "default" : "outline"}
                    className="min-h-11"
                    aria-pressed={view === item.value}
                    onClick={() => setView(item.value)}
                  >
                    {t(item.label)}
                  </Button>
                ))}
                <Button variant="outline" className="min-h-11" disabled>
                  {t("queueFilterMentions")}
                </Button>
              </div>
              {view === CustomerInboxView.CustomerInboxViewCoworkers ? (
                <MobileCustomerAssignee value={assignee} onChange={setAssignee} />
              ) : null}
              <div className="space-y-2">
                <label className="block text-sm font-medium" htmlFor="mobile-inbox-channel">
                  {t("filterChannel")}
                </label>
                <select
                  id="mobile-inbox-channel"
                  className="h-11 w-full rounded-md border bg-background px-3 text-sm"
                  value={channel}
                  onChange={(event) => setChannel(event.target.value)}
                >
                  <option value="">{t("filterAllChannels")}</option>
                  {channels.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.enabled
                        ? item.name
                        : `${item.name}（${t("filterChannelDisabled")}）`}
                    </option>
                  ))}
                </select>
              </div>
              <div className="space-y-2">
                <label className="block text-sm font-medium" htmlFor="mobile-inbox-status">
                  {t("filterServiceStatus")}
                </label>
                <select
                  id="mobile-inbox-status"
                  className="h-11 w-full rounded-md border bg-background px-3 text-sm"
                  value={status}
                  onChange={(event) => setStatus(event.target.value as ServiceSessionStatus)}
                >
                  <option value={ServiceSessionStatus.ServiceSessionStatusOpen}>
                    {t("filterServiceStatusOpen")}
                  </option>
                  <option value={ServiceSessionStatus.ServiceSessionStatusClosed}>
                    {t("filterServiceStatusClosed")}
                  </option>
                </select>
              </div>
            </div>
          ) : (
            <fieldset className="space-y-2">
              <legend className="pb-2 text-sm font-medium">{t("filterKind")}</legend>
              {options.map((option) => (
                <label key={option.kind} className="flex min-h-11 items-center gap-3 text-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={kinds.includes(option.kind)}
                    onChange={(event) =>
                      setKinds(toggleInboxKinds(query.scope, kinds, option.kind, event.target.checked))
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
                customer
                  ? {
                      customerView: view,
                      assigneeIdentityId:
                        view === CustomerInboxView.CustomerInboxViewCoworkers ? assignee : "",
                      channelId: channel,
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
