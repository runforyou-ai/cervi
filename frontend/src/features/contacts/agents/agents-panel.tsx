/** AI 员工列表、筛选、配置入口和状态管理面板。 */
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

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
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { PageHeader } from "@/components/page-header"
import {
  ResourceTable,
  ResourceTableActions,
} from "@/components/resource-table"
import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { TableCell } from "@/components/ui/table"
import { WorkStatusBadge } from "@/components/work-status"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactListLayout } from "@/features/contacts/contact-list-layout"
import { ContactScopeMobileSelect } from "@/features/contacts/contact-scope-mobile-select"
import { userStatusLabel } from "@/features/contacts/external/contact-labels"
import { JoinedTeamsCell } from "@/features/contacts/joined-teams-cell"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { UserStatusBadge } from "@/features/contacts/user-status-badge"
import { roleDisplayName } from "@/lib/role-labels"
import { useDateTime } from "@/hooks/use-date-time"
import { useContactInvalidator } from "@/features/contacts/use-contact-invalidator"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
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
  const { formatDateTime } = useDateTime()
  const invalidateContact = useContactInvalidator()
  const { searchParams, setParameters, query, search, setSearch, currentPage } =
    useContactSearch()
  const status =
    optionalWailsEnum(UserStatus, searchParams.get("status")) ??
    UserStatus.UserStatusActive
  const [changingAgentStatus, setChangingAgentStatus] =
    useState<AgentListItemData | null>(null)
  const [deleting, setDeleting] = useState(false)

  const list = useResource(
    resourceKeys.agents({ query, status, page: currentPage, pageSize: 50 }),
    () => listAgents({ query, status, page: currentPage, pageSize: 50 }),
  )
  const agents = list.data?.agents ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  /** 禁用 AI 员工账号或恢复为正常状态。 */
  async function changeAgentStatus() {
    if (!changingAgentStatus) return
    setDeleting(true)
    try {
      const saved =
        changingAgentStatus.status === UserStatus.UserStatusActive
          ? await deactivateAgent(changingAgentStatus.id)
          : await reactivateAgent(changingAgentStatus.id)
      console.info("AI 员工账号状态已修改", {
        identity_id: saved.identityId,
        agent_id: saved.id,
        status: saved.status,
      })
      toast.success(
        t(
          changingAgentStatus.status === UserStatus.UserStatusActive
            ? "agents.status.deactivated"
            : "agents.status.reactivated",
        ),
      )
      setChangingAgentStatus(null)
      void invalidateContact("agent", saved.id)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("修改 AI 员工状态失败", {
        agent_id: changingAgentStatus.id,
        error,
      })
      toast.error(t("agents.status.error"))
    } finally {
      setDeleting(false)
    }
  }

  const hasInternalFilters = Boolean(status !== UserStatus.UserStatusActive)

  return (
    <>
      <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <PageHeader
          title={t("scopes.agents")}
          beforeTitle={
            <ContactScopeMobileSelect
              scope="agents"
              teams={teams}
              channels={channels}
            />
          }
        />

        <ListToolbar>
          <ListToolbarSearch
            value={search}
            aria-label={t("search.agents")}
            onChange={(event) => setSearch(event.target.value)}
          />
          <ListToolbarFilter
            label={t("filters.accountStatus")}
            value={status}
            options={[
              {
                value: UserStatus.UserStatusActive,
                label: t("statuses.active"),
              },
              {
                value: UserStatus.UserStatusInactive,
                label: t("statuses.inactive"),
              },
            ]}
            onValueChange={(value) =>
              setParameters({
                status: value === UserStatus.UserStatusActive ? null : value,
                page: null,
                selected: null,
              })
            }
          />
          {hasInternalFilters ? (
            <ListToolbarReset
              onClick={() =>
                setParameters({
                  status: null,
                  roleId: null,
                  page: null,
                })
              }
            >
              {t("filters.clear")}
            </ListToolbarReset>
          ) : null}
        </ListToolbar>

        <ContactListLayout
          loading={list.loading}
          error={Boolean(list.error)}
          onRetry={() => void list.refresh()}
          page={page}
          onPageChange={(number) =>
            setParameters({ page: String(number), selected: null })
          }
        >
          <ResourceTable
            columns={[
              { key: "name", header: t("columns.name") },
              { key: "role", header: t("columns.role") },
              { key: "joinedTeams", header: t("columns.joinedTeams") },
              { key: "model", header: t("columns.model") },
              { key: "accountStatus", header: t("columns.accountStatus") },
              { key: "workStatus", header: t("columns.workStatus") },
              { key: "createdAt", header: t("columns.createdAt") },
              {
                key: "actions",
                header: tCommon("table.actions"),
                className: "w-px",
              },
            ]}
            rows={agents}
            rowKey={(agent) => agent.id}
            empty={t("list.empty")}
          >
            {(agent) => (
              <>
                <TableCell className="font-medium">
                  {agent.displayName}
                </TableCell>
                <TableCell>{roleDisplayName(agent.role, tCommon)}</TableCell>
                <TableCell className="max-w-xs">
                  <JoinedTeamsCell teams={agent.teams} />
                </TableCell>
                <TableCell className="max-w-xs">
                  <span className="block truncate">
                    {agent.execution.managed.providerName} ·{" "}
                    {agent.execution.managed.modelName}
                  </span>
                </TableCell>
                <TableCell>
                  <UserStatusBadge
                    status={agent.status}
                    label={userStatusLabel(agent.status, t)}
                  />
                </TableCell>
                <TableCell>
                  <WorkStatusBadge status={agent.workStatus} />
                </TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">
                  {formatDateTime(agent.createdAt)}
                </TableCell>
                <ResourceTableActions
                  menu={
                    <DropdownMenuItem
                      destructive={
                        agent.status === UserStatus.UserStatusActive
                      }
                      onSelect={() => setChangingAgentStatus(agent)}
                    >
                      {t(
                        agent.status === UserStatus.UserStatusActive
                          ? "agents.status.deactivate"
                          : "agents.status.reactivate",
                      )}
                    </DropdownMenuItem>
                  }
                >
                  <Button
                    size="sm"
                    disabled={agent.status !== UserStatus.UserStatusActive}
                    onClick={() =>
                      navigate(
                        `/inbox?scope=internal&target=${agent.identityId}`,
                      )
                    }
                  >
                    {t("sendMessage")}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      navigate(
                        `/contacts/ai-employees/${agent.id}?tab=basic&returnTo=${encodeURIComponent(location.pathname + location.search)}`,
                      )
                    }
                  >
                    {t("agents.configure")}
                  </Button>
                </ResourceTableActions>
              </>
            )}
          </ResourceTable>
        </ContactListLayout>
      </section>

      <ContactCreateDialogs
        scope="agents"
        channels={channels}
        roles={roles}
        teams={teams}
        searchParams={searchParams}
        setParameters={setParameters}
      />

      <AlertDialog
        open={changingAgentStatus !== null}
        onOpenChange={(open) => !open && setChangingAgentStatus(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(
                changingAgentStatus?.status === UserStatus.UserStatusActive
                  ? "agents.status.deactivateTitle"
                  : "agents.status.reactivateTitle",
                { name: changingAgentStatus?.displayName ?? "" },
              )}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                changingAgentStatus?.status === UserStatus.UserStatusActive
                  ? "agents.status.deactivateDescription"
                  : "agents.status.reactivateDescription",
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void changeAgentStatus()}
            >
              {deleting
                ? t("agents.status.saving")
                : tCommon("actions.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
