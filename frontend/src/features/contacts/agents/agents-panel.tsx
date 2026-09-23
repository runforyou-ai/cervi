/** AI 员工列表、筛选、配置入口和状态管理面板。 */
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"

import {
  UserStatus,
  deactivateAgent,
  listAgents,
  reactivateAgent,
  type AgentListItemData,
  type ChannelOption,
  type RoleData,
  type Team,
} from "@/api"
import {
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import {
  AccountStatusFilter,
  useAccountStatusToggle,
} from "@/features/contacts/account-status-toggle"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactListSection } from "@/features/contacts/contact-list-section"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { roleDisplayName } from "@/lib/role-labels"
import { contactResourceKeys } from "@/features/contacts/use-contact-invalidator"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 显示 AI 员工目录并提供配置和状态操作。 */
export function AgentsPanel({
  channels,
  roles,
  teams,
}: {
  channels: ChannelOption[]
  roles: RoleData[]
  teams: Team[]
}) {
  const { t } = useTranslation("contacts")
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const location = useLocation()
  const { searchParams, setParameters, query, search, setSearch, currentPage } =
    useContactSearch()
  const status =
    optionalWailsEnum(UserStatus, searchParams.get("status")) ??
    UserStatus.UserStatusActive
  const statusToggle = useAccountStatusToggle<AgentListItemData>({
    scope: "agents",
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

  const hasInternalFilters = Boolean(status !== UserStatus.UserStatusActive)

  return (
    <>
      <ContactListSection
        title={t("scopes.agents")}
        description={t("scopeDescriptions.agents")}
        scope={{ scope: "agents", teams, channels }}
        toolbar={
          <>
            <ListToolbarSearch
              value={search}
              aria-label={t("search.agents")}
              onChange={(event) => setSearch(event.target.value)}
            />
            <AccountStatusFilter value={status} setParameters={setParameters} />
            {hasInternalFilters ? (
              <ListToolbarReset
                onClick={() =>
                  setParameters({
                    status: null,
                    page: null,
                  })
                }
              >
                {tCommon("actions.clearFilters")}
              </ListToolbarReset>
            ) : null}
          </>
        }
        list={list}
        page={page}
        setParameters={setParameters}
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
                  secondary={roleDisplayName(agent.role, tCommon)}
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
          ]}
          rows={agents}
          rowKey={(agent) => agent.id}
          empty={t("list.empty")}
          onRowActivate={(agent) =>
            navigate(
              `/contacts/ai-employees/${agent.id}?tab=basic&returnTo=${encodeURIComponent(location.pathname + location.search)}`,
            )
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
      </ContactListSection>

      <ContactCreateDialogs
        scope="agents"
        channels={channels}
        roles={roles}
        teams={teams}
        searchParams={searchParams}
        setParameters={setParameters}
      />

      <ConfirmationDialog {...statusToggle.dialog} />
    </>
  )
}
