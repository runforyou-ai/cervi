/** AI 员工列表页：筛选、配置入口和状态管理。 */
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useNavigate } from "react-router"

import {
  UserStatus,
  deactivateAgent,
  listAgents,
  reactivateAgent,
  type AgentListItemData,
} from "@/api"
import {
  ListToolbar,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { PageHeader } from "@/components/page-header"
import { ResourceListLayout } from "@/components/resource-list"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { Button } from "@/components/ui/button"
import {
  AccountStatusFilter,
  useAccountStatusToggle,
} from "@/features/contacts/account-status-toggle"
import { contactResourceKeys } from "@/features/contacts/use-contact-invalidator"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { useResource } from "@/hooks/use-resource"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 显示 AI 员工列表并提供配置和状态操作。 */
export function AgentListPage() {
  const { t } = useTranslation(["agents", "common"])
  const { formatDateTime } = useDateTime()
  const navigate = useNavigate()
  const location = useLocation()
  const { searchParams, setParameters, query, search, setSearch, currentPage } =
    useContactSearch()
  const status =
    optionalWailsEnum(UserStatus, searchParams.get("status")) ??
    UserStatus.UserStatusActive
  const statusToggle = useAccountStatusToggle<AgentListItemData>({
    keyPrefix: "agents:status",
    deactivate: deactivateAgent,
    reactivate: reactivateAgent,
    invalidateKeys: (agent) => contactResourceKeys("agent", agent.id),
    logLabel: "修改 AI 员工状态",
  })

  const list = useResource(
    resourceKeys.agents({ query, status, page: currentPage, pageSize: 50 }),
    () => listAgents({ query, status, page: currentPage, pageSize: 50 }),
  )
  const agents = list.data?.agents ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }
  const returnTo = encodeURIComponent(location.pathname + location.search)

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("title")} description={t("description")}>
        <Button variant="ghost" size="icon-sm" asChild>
          <Link
            to={`/ai-employees/new?returnTo=${returnTo}`}
            aria-label={t("create")}
            title={t("create")}
          >
            <PlusIcon />
          </Link>
        </Button>
      </PageHeader>

      <ListToolbar>
        <ListToolbarSearch
          value={search}
          aria-label={t("search")}
          onChange={(event) => setSearch(event.target.value)}
        />
        <AccountStatusFilter value={status} setParameters={setParameters} />
        {status !== UserStatus.UserStatusActive ? (
          <ListToolbarReset
            onClick={() => setParameters({ status: null, page: null })}
          >
            {t("common:actions.clearFilters")}
          </ListToolbarReset>
        ) : null}
      </ListToolbar>

      <ResourceListLayout
        resources={list}
        errorMessage={t("loadError")}
        page={page}
        onPageChange={(number) => setParameters({ page: String(number) })}
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "name",
              header: t("columns.name"),
              cell: (agent) => (
                <ResourceRowIdentity
                  avatar={{
                    imageURL: agent.avatarUrl,
                    name: agent.displayName,
                    fallback: "agent",
                  }}
                  status={agent.workStatus}
                  name={agent.displayName}
                />
              ),
            },
            {
              key: "model",
              header: t("columns.model"),
              cellClassName: "max-w-xs text-muted-foreground",
              cell: (agent) => (
                <span className="block truncate">
                  {agent.execution.managed.providerName} ·{" "}
                  {agent.execution.managed.modelName}
                </span>
              ),
            },
            {
              key: "time",
              header: t("columns.addedAt"),
              cellClassName: "w-px whitespace-nowrap text-right text-muted-foreground",
              cell: (agent) =>
                t("addedAt", { time: formatDateTime(agent.createdAt) }),
            },
          ]}
          rows={agents}
          rowKey={(agent) => agent.id}
          empty={t("empty")}
          onRowActivate={(agent) =>
            navigate(`/ai-employees/${agent.id}?tab=basic&returnTo=${returnTo}`)
          }
          // 已停用的 AI 员工保留禁用的发消息。
          rowActions={(agent) => [
            {
              key: "message",
              label: t("sendMessage"),
              disabled: agent.status !== UserStatus.UserStatusActive,
              onSelect: () =>
                navigate(`/chats?target=${agent.identityId}`),
            },
            statusToggle.rowAction(agent),
          ]}
        />
      </ResourceListLayout>

      <ConfirmationDialog {...statusToggle.dialog} />
    </section>
  )
}
