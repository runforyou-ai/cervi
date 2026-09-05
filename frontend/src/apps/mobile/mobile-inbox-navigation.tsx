/** 移动端消息分类、地址查询和客户筛选面板。 */
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import {
  CustomerInboxView,
  InboxScope,
  listCustomerServiceAssignees,
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
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { optionalWailsEnum } from "@/lib/wails-enum"
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
  {
    value: CustomerInboxView.CustomerInboxViewClosed,
    label: "queueFilterClosed",
  },
] as const

/** 从地址派生与服务端及桌面端一致的完整消息查询。 */
export function useMobileInboxQuery() {
  const [params, setParams] = useSearchParams()
  const scope =
    optionalWailsEnum(InboxScope, params.get("scope")) ??
    InboxScope.InboxScopeAll
  const customerView =
    scope === InboxScope.InboxScopeCustomer
      ? (optionalWailsEnum(CustomerInboxView, params.get("view")) ??
        CustomerInboxView.CustomerInboxViewQueue)
      : CustomerInboxView.CustomerInboxViewQueue
  const assigneeIdentityId =
    customerView === CustomerInboxView.CustomerInboxViewCoworkers
      ? (params.get("assignee") ?? "")
      : ""
  const query = { scope, customerView, assigneeIdentityId }

  /** 更换筛选时替换当前列表地址，不增加返回层级。 */
  function changeQuery(changes: LoadInboxQuery) {
    const next = { ...query, ...changes }
    const search = new URLSearchParams()
    if (next.scope !== InboxScope.InboxScopeAll) search.set("scope", next.scope)
    if (next.scope === InboxScope.InboxScopeCustomer) {
      if (next.customerView !== CustomerInboxView.CustomerInboxViewQueue)
        search.set("view", next.customerView)
      if (
        next.customerView === CustomerInboxView.CustomerInboxViewCoworkers &&
        next.assigneeIdentityId
      )
        search.set("assignee", next.assigneeIdentityId)
    }
    setParams(search, { replace: true })
  }
  return { query, changeQuery }
}

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

/** 按需加载可选同事，使用原生选择控件避免叠加业务面板。 */
function MobileCustomerAssignee({
  value,
  onChange,
}: {
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation("mobile")
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
        <p className="text-xs text-muted-foreground">{t("loading")}</p>
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

/** 在底部面板中选择客户视图与负责人，取消时保留原筛选。 */
export function MobileCustomerFilter({
  query,
  onChange,
}: {
  query: Required<LoadInboxQuery>
  onChange: (query: LoadInboxQuery) => void
}) {
  const { t } = useTranslation("inbox")
  const { t: tMobile } = useTranslation("mobile")
  const [open, setOpen] = useState(false)
  const [view, setView] = useState(query.customerView)
  const [assignee, setAssignee] = useState(query.assigneeIdentityId)
  const { data } = useResource(
    resourceKeys.customerServiceAssignees(),
    listCustomerServiceAssignees,
    { enabled: Boolean(query.assigneeIdentityId), staleTime: 0 },
  )
  const selected = data?.find(
    (item) => item.identityId === query.assigneeIdentityId,
  )
  const viewLabel = customerViews.find(
    (item) => item.value === query.customerView,
  )!.label
  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (next) {
          setView(query.customerView)
          setAssignee(query.assigneeIdentityId)
        }
        setOpen(next)
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
            {t(viewLabel)}
            {query.assigneeIdentityId
              ? ` · ${selected?.displayName ?? tMobile("inbox.selectedAssignee")}`
              : ""}
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
          <SheetTitle className="flex-1">{t("queueFilterLabel")}</SheetTitle>
          <SheetClose asChild>
            <Button variant="ghost" className="min-h-11">
              {tMobile("cancel")}
            </Button>
          </SheetClose>
        </SheetHeader>
        <div className="overflow-y-auto p-4 space-y-9">
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
            </div>
            {view === CustomerInboxView.CustomerInboxViewCoworkers ? (
              <MobileCustomerAssignee value={assignee} onChange={setAssignee} />
            ) : null}
          </div>
          <Button
            className="min-h-11 w-full"
            onClick={() => {
              onChange({
                customerView: view,
                assigneeIdentityId:
                  view === CustomerInboxView.CustomerInboxViewCoworkers
                    ? assignee
                    : "",
              })
              setOpen(false)
            }}
          >
            {tMobile("apply")}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
