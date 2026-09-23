/** 团队列表：新建、进入团队成员页、编辑与删除团队。 */
import { useState } from "react"
import { PlusIcon, UsersIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"

import {
  deleteTeam,
  listTeams,
  type ChannelOption,
  type RoleData,
  type Team,
} from "@/api"
import { ListToolbarSearch } from "@/components/list-toolbar"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactListSection } from "@/features/contacts/contact-list-section"
import { TeamForm } from "@/features/contacts/teams/team-form"
import { teamMembershipCacheKeys } from "@/features/contacts/teams/team-membership-cache"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

/** 列出企业团队并提供团队维护入口。 */
export function TeamListPanel({
  channels,
  roles,
  teams,
}: {
  channels: ChannelOption[]
  roles: RoleData[]
  teams: Team[]
}) {
  const { t } = useTranslation(["contacts", "common"])
  const navigate = useNavigate()
  const location = useLocation()
  const invalidate = useResourceInvalidator()
  const { searchParams, setParameters, query, search, setSearch, currentPage } =
    useContactSearch()
  const [editingTeam, setEditingTeam] = useState<Team | null>(null)

  const list = useResource(
    resourceKeys.teams({ query, page: currentPage, pageSize: 50 }),
    () => listTeams({ query, page: currentPage, pageSize: 50 }),
  )
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  const teamDeletion = useConfirmedAction<Team>({
    action: (team) => deleteTeam(team.id),
    invalidateKeys: () => [resourceKeys.teams(), ...teamMembershipCacheKeys],
    successMessage: () => t("teams.delete.success"),
    errorMessage: () => t("teams.delete.error"),
    logLabel: "删除团队",
  })

  return (
    <>
      <ContactListSection
        title={t("scopes.teams")}
        description={t("scopeDescriptions.teamList")}
        scope="team"
        headerActions={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t("teams.create")}
            title={t("teams.create")}
            onClick={() => setParameters({ newTeam: "1" })}
          >
            <PlusIcon />
          </Button>
        }
        toolbar={
          <ListToolbarSearch
            value={search}
            aria-label={t("search.teams")}
            onChange={(event) => setSearch(event.target.value)}
          />
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
              header: t("teams.form.name"),
              cellClassName: "min-w-0",
              cell: (team) => (
                <ResourceRowIdentity
                  icon={UsersIcon}
                  name={team.name}
                  secondary={t("teams.memberCount", { count: team.memberCount })}
                  description={team.description || undefined}
                />
              ),
            },
          ]}
          rows={list.data?.teams ?? []}
          rowKey={(team) => team.id}
          empty={query ? t("teams.emptyFiltered") : t("teams.empty")}
          onRowActivate={(team) =>
            navigate(
              `/contacts/teams/${team.id}?returnTo=${encodeURIComponent(location.pathname + location.search)}`,
            )
          }
          rowActions={(team) => [
            {
              key: "edit",
              label: t("common:actions.edit"),
              onSelect: () => setEditingTeam(team),
            },
            {
              key: "delete",
              label: t("common:actions.delete"),
              destructive: true,
              separatorBefore: true,
              onSelect: () => teamDeletion.select(team),
            },
          ]}
        />
      </ContactListSection>

      <ContactCreateDialogs
        scope="team"
        channels={channels}
        roles={roles}
        teams={teams}
        searchParams={searchParams}
        setParameters={setParameters}
      />

      <Dialog
        open={editingTeam !== null}
        onOpenChange={(open) => !open && setEditingTeam(null)}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("teams.edit")}</DialogTitle>
            <DialogDescription>{t("teams.editDescription")}</DialogDescription>
          </DialogHeader>
          {editingTeam ? (
            <TeamForm
              team={editingTeam}
              onSaved={() => {
                void invalidate(resourceKeys.teams())
                for (const key of teamMembershipCacheKeys) void invalidate(key)
                setEditingTeam(null)
              }}
              onCancel={() => setEditingTeam(null)}
            />
          ) : null}
        </DialogContent>
      </Dialog>

      <ConfirmationDialog
        {...teamDeletion.dialog}
        title={t("teams.delete.title", { name: teamDeletion.item?.name ?? "" })}
        description={t("teams.delete.description", {
          count: teamDeletion.item?.memberCount ?? 0,
        })}
        pendingLabel={t("common:actions.deleting")}
      />
    </>
  )
}
