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
import { ListActionButton } from "@/components/list-action-button"
import { PageHeader } from "@/components/page-header"
import { ProfileAvatar } from "@/components/profile-avatar"
import { ResourceListLayout } from "@/components/resource-list"
import { ResourceTable } from "@/components/resource-table"
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
import { WorkStatusDot } from "@/components/work-status"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactScopeMobileSelect } from "@/features/contacts/contact-scope-mobile-select"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { roleDisplayName } from "@/lib/role-labels"
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
          description={t("scopeDescriptions.agents")}
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

        <ResourceListLayout
          loading={list.loading}
          error={Boolean(list.error)}
          errorMessage={t("list.loadError")}
          onRetry={() => void list.refresh()}
          page={page}
          onPageChange={(number) =>
            setParameters({ page: String(number), selected: null })
          }
        >
          <ResourceTable
            hideHeader
            columns={[
              {
                key: "name",
                header: t("columns.name"),
                cell: (agent) => (
                  <div className="flex min-w-0 items-center gap-3">
                    <span className="relative size-9 shrink-0">
                      <ProfileAvatar
                        imageURL={agent.avatarUrl}
                        name={agent.displayName}
                        fallback="agent"
                        className="size-full"
                      />
                      <WorkStatusDot
                        status={agent.workStatus}
                        className="absolute -right-0.5 -bottom-0.5 ring-2 ring-background"
                      />
                    </span>
                    <span className="truncate">
                      <span className="font-medium">{agent.displayName}</span>
                      <span aria-hidden="true" className="mx-1.5 text-muted-foreground">·</span>
                      <span className="text-muted-foreground">
                        {roleDisplayName(agent.role, tCommon)}
                      </span>
                    </span>
                  </div>
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
            actions={(agent) => ({
              // 操作靠右对齐，不显示发消息的行中禁用按钮与其他行右端对齐。
              primary: (
                <div className="ml-auto flex items-center gap-1">
                  {agent.status === UserStatus.UserStatusActive ? (
                    <ListActionButton
                      onClick={() =>
                        navigate(`/inbox?scope=internal&target=${agent.identityId}`)
                      }
                    >
                      {t("sendMessage")}
                    </ListActionButton>
                  ) : null}
                  <ListActionButton
                    tone={
                      agent.status === UserStatus.UserStatusActive
                        ? "destructive"
                        : "success"
                    }
                    onClick={() => setChangingAgentStatus(agent)}
                  >
                    {t(
                      agent.status === UserStatus.UserStatusActive
                        ? "agents.status.deactivate"
                        : "agents.status.reactivate",
                    )}
                  </ListActionButton>
                </div>
              ),
            })}
          />
        </ResourceListLayout>
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
