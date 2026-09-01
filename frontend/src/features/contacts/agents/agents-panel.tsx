/** AI 员工列表、筛选、详情和账号状态管理面板。 */
import { useEffect, useState } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  UserStatus,
  deactivateAgent,
  getAgent,
  isApiError,
  listAgents,
  reactivateAgent,
  sessionPath,
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { WorkStatusBadge } from "@/components/work-status"
import { AgentDetailView } from "@/features/contacts/agents/agent-detail"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactDetailSheet } from "@/features/contacts/contact-detail-sheet"
import { ContactListLayout } from "@/features/contacts/contact-list-layout"
import { ContactScopeMobileSelect } from "@/features/contacts/contact-scope-mobile-select"
import { userStatusLabel } from "@/features/contacts/external/contact-labels"
import { JoinedTeamsCell } from "@/features/contacts/joined-teams-cell"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { UserStatusBadge } from "@/features/contacts/user-status-badge"
import { roleDisplayName } from "@/features/roles/role-labels"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** AI 员工范围的列表、详情和弹窗。 */
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
  const { formatDateTime } = useDateTime()
  const invalidate = useResourceInvalidator()
  const {
    searchParams,
    setParameters,
    query,
    search,
    setSearch,
    currentPage,
    selected,
  } = useContactSearch()
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

  const detail = useResource(resourceKeys.agent(selected), () => getAgent(selected), {
    enabled: Boolean(selected),
  })
  const detailAgent = selected ? (detail.data ?? null) : null

  const detailError = detail.error
  useEffect(() => {
    if (!selected || !detailError) return
    if (isApiError(detailError) && sessionPath(detailError.state)) return
    console.warn("联系人详情加载失败", detailError)
    toast.error(t("detail.loadError"))
    setParameters({ selected: null })
  }, [detailError, selected, setParameters, t])

  /** 关闭 AI 员工详情。 */
  function closeDetail() {
    setParameters({ selected: null, new: null })
  }

  /** 刷新列表并关闭详情。 */
  function refreshAndClose() {
    closeDetail()
    void invalidate(resourceKeys.agents())
  }

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
      void invalidate(resourceKeys.agent(saved.id))
      void invalidate(resourceKeys.agents())
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
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>{t("columns.name")}</TableHead>
                <TableHead>{t("columns.role")}</TableHead>
                <TableHead>{t("columns.joinedTeams")}</TableHead>
                <TableHead>{t("columns.model")}</TableHead>
                <TableHead>{t("columns.accountStatus")}</TableHead>
                <TableHead>{t("columns.workStatus")}</TableHead>
                <TableHead>{t("columns.createdAt")}</TableHead>
                <TableHead className="text-right">
                  {t("columns.actions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {agents.map((agent) => (
                <TableRow key={agent.id}>
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
                  <TableCell className="text-right whitespace-nowrap">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setParameters({ selected: agent.id })}
                      >
                        {t("detail.action")}
                      </Button>
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={t("list.more")}
                            title={t("list.more")}
                          >
                            <MoreHorizontalIcon />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
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
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {agents.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell
                    colSpan={8}
                    className="h-32 text-center text-muted-foreground"
                  >
                    {t("list.empty")}
                  </TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>
        </ContactListLayout>
      </section>

      <ContactDetailSheet
        open={Boolean(selected)}
        onClose={closeDetail}
        title={detailAgent?.displayName ?? t("detail.agentTitle")}
        description={t("detail.agentDescription")}
        loading={detail.loading && Boolean(selected)}
      >
        {detailAgent ? (
          <AgentDetailView
            key={detailAgent.id}
            agent={detailAgent}
            roles={roles}
            teams={teams}
            onSaved={(saved) => {
              void invalidate(resourceKeys.agent(saved.id))
              void invalidate(resourceKeys.agents())
            }}
            onNotFound={refreshAndClose}
          />
        ) : null}
      </ContactDetailSheet>

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
            <AlertDialogCancel>{t("agents.status.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void changeAgentStatus()}
            >
              {deleting
                ? t("agents.status.saving")
                : t("agents.status.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
