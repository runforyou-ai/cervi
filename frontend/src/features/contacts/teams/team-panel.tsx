/** 团队成员列表、批量管理和团队维护面板。 */
import { useEffect, useState } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  OrganizationIdentityType,
  WorkStatus,
  deleteTeam,
  listTeamMembers,
  removeTeamMembers,
  type ChannelOption,
  type RoleData,
  type Team,
  type TeamMember,
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
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
import { WorkStatusBadge, workStatusLabel } from "@/components/work-status"
import { useWorkspace } from "@/contexts/workspace-context"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactListLayout } from "@/features/contacts/contact-list-layout"
import { ContactScopeMobileSelect } from "@/features/contacts/contact-scope-mobile-select"
import { TeamForm } from "@/features/contacts/teams/team-form"
import { TeamMemberPicker } from "@/features/contacts/teams/team-member-picker"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 单个团队范围的成员列表、批量操作和团队弹窗。 */
export function TeamPanel({
  channels,
  roles,
  teams,
  teamId,
}: {
  channels: ChannelOption[]
  roles: RoleData[]
  teams: Team[]
  teamId: string
}) {
  const { t } = useTranslation("contacts")
  const { t: tCommon } = useTranslation("common")
  const { identity } = useWorkspace()
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
  } = useContactSearch()
  const workStatus = optionalWailsEnum(
    WorkStatus,
    searchParams.get("workStatus"),
  )
  const editingTeam = searchParams.get("editTeam") === "1"
  const addingTeamMembers = searchParams.get("addMembers") === "1"
  const selectedTeam = teams.find((team) => team.id === teamId)
  const [deletingTeam, setDeletingTeam] = useState<Team | null>(null)
  const [removingTeamMembers, setRemovingTeamMembers] = useState<TeamMember[]>(
    [],
  )
  const [selectedTeamMemberIdentityIDs, setSelectedTeamMemberIdentityIDs] =
    useState<Set<string>>(new Set())
  const [deleting, setDeleting] = useState(false)

  const list = useResource(
    resourceKeys.teamMembers(teamId, { query, workStatus, page: currentPage, pageSize: 50 }),
    () =>
      listTeamMembers(teamId, {
        query,
        workStatus,
        page: currentPage,
        pageSize: 50,
      }),
  )
  const teamMembers = list.data?.members ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  useEffect(() => {
    setSelectedTeamMemberIdentityIDs(new Set())
  }, [currentPage, query, teamId, workStatus])

  /** 团队或成员关系变化后，失效内嵌所属团队的成员和 AI 员工缓存。 */
  function invalidateMembershipCaches() {
    void invalidate(resourceKeys.users())
    void invalidate(resourceKeys.user())
    void invalidate(resourceKeys.agents())
    void invalidate(resourceKeys.agent())
  }

  /** 当前用户使用工作台中的即时状态，其他团队成员使用列表结果。 */
  function identityWorkStatus(member: TeamMember) {
    return member.identityType ===
      OrganizationIdentityType.OrganizationIdentityTypeUser &&
      member.identityId === identity.user.identityId
      ? identity.user.workStatus
      : member.workStatus
  }

  /** 删除当前团队并刷新团队列表。 */
  async function removeCurrentTeam() {
    if (!deletingTeam) return
    const deletingTeamID = deletingTeam.id
    setDeleting(true)
    try {
      await deleteTeam(deletingTeamID)
      void invalidate(resourceKeys.teams())
      invalidateMembershipCaches()
      setDeletingTeam(null)
      toast.success(t("teams.delete.success"))
      if (teamId === deletingTeamID) {
        navigate("/contacts/employees", { replace: true })
      }
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("删除团队失败", error)
      toast.error(t("teams.delete.error"))
    } finally {
      setDeleting(false)
    }
  }

  /** 将选中的团队成员批量移出当前团队。 */
  async function removeMembersFromCurrentTeam() {
    if (!selectedTeam || removingTeamMembers.length === 0) return
    setDeleting(true)
    try {
      await removeTeamMembers(selectedTeam.id, {
        members: removingTeamMembers.map((member) => ({
          identityType: member.identityType,
          identityId: member.identityId,
        })),
      })
      void invalidate(resourceKeys.teams())
      toast.success(
        t(
          removingTeamMembers.length === 1
            ? "teams.members.removed"
            : "teams.members.removedMultiple",
          { count: removingTeamMembers.length },
        ),
      )
      setRemovingTeamMembers([])
      setSelectedTeamMemberIdentityIDs(new Set())
      void invalidate(resourceKeys.teamMembers(teamId))
      void invalidate(resourceKeys.teamMemberCandidates(teamId))
      invalidateMembershipCaches()
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("移出团队成员失败", error)
      toast.error(t("teams.members.removeError"))
    } finally {
      setDeleting(false)
    }
  }

  /** 切换当前页所有团队成员的选中状态。 */
  function toggleAllVisibleTeamMembers(checked: boolean) {
    setSelectedTeamMemberIdentityIDs(
      checked
        ? new Set(teamMembers.map((member) => member.identityId))
        : new Set(),
    )
  }

  /** 切换单个团队成员的选中状态。 */
  function toggleTeamMember(identityID: string, checked: boolean) {
    setSelectedTeamMemberIdentityIDs((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(identityID)
      } else {
        next.delete(identityID)
      }
      return next
    })
  }

  const hasInternalFilters = Boolean(workStatus)
  const allVisibleTeamMembersSelected =
    teamMembers.length > 0 &&
    teamMembers.every((member) =>
      selectedTeamMemberIdentityIDs.has(member.identityId),
    )

  return (
    <>
      <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <PageHeader
          title={selectedTeam?.name ?? t("scopes.teams")}
          beforeTitle={
            <ContactScopeMobileSelect
              scope="team"
              teamId={teamId}
              teams={teams}
              channels={channels}
            />
          }
        >
          {selectedTeam ? (
            <>
              {selectedTeamMemberIdentityIDs.size > 0 ? (
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() =>
                    setRemovingTeamMembers(
                      teamMembers.filter((member) =>
                        selectedTeamMemberIdentityIDs.has(member.identityId),
                      ),
                    )
                  }
                >
                  {t("teams.members.removeSelected", {
                    count: selectedTeamMemberIdentityIDs.size,
                  })}
                </Button>
              ) : null}
              <Button
                size="sm"
                onClick={() => setParameters({ addMembers: "1" })}
              >
                {t("teams.members.add")}
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t("teams.more")}
                    title={t("teams.more")}
                  >
                    <MoreHorizontalIcon />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    onSelect={() => setParameters({ editTeam: "1" })}
                  >
                    {t("teams.edit")}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    destructive
                    onSelect={() => setDeletingTeam(selectedTeam)}
                  >
                    {t("teams.delete.action")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          ) : null}
        </PageHeader>

        <ListToolbar>
          <ListToolbarSearch
            value={search}
            aria-label={t("search.teamMembers")}
            onChange={(event) => setSearch(event.target.value)}
          />
          <ListToolbarFilter
            label={t("filters.workStatus")}
            allLabel={t("filters.allWorkStatuses")}
            value={workStatus ?? ""}
            options={[
              {
                value: WorkStatus.WorkStatusWorking,
                label: workStatusLabel(WorkStatus.WorkStatusWorking, tCommon),
              },
              {
                value: WorkStatus.WorkStatusAway,
                label: workStatusLabel(WorkStatus.WorkStatusAway, tCommon),
              },
              {
                value: WorkStatus.WorkStatusOffDuty,
                label: workStatusLabel(WorkStatus.WorkStatusOffDuty, tCommon),
              },
            ]}
            onValueChange={(value) =>
              setParameters({
                workStatus: value || null,
                page: null,
                selected: null,
              })
            }
          />
          {hasInternalFilters ? (
            <ListToolbarReset
              onClick={() =>
                setParameters({
                  workStatus: null,
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
                <TableHead className="w-10">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    aria-label={t("teams.members.selectAll")}
                    checked={allVisibleTeamMembersSelected}
                    onChange={(event) =>
                      toggleAllVisibleTeamMembers(event.target.checked)
                    }
                  />
                </TableHead>
                <TableHead>{t("columns.memberName")}</TableHead>
                <TableHead>{t("columns.type")}</TableHead>
                <TableHead>{t("columns.workStatus")}</TableHead>
                <TableHead>{t("columns.joinedAt")}</TableHead>
                <TableHead className="text-right">
                  {t("columns.actions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {teamMembers.map((member) => (
                <TableRow key={member.identityId}>
                  <TableCell>
                    <input
                      type="checkbox"
                      className="size-4 accent-primary"
                      aria-label={t("teams.members.selectMember", {
                        name: member.displayName,
                      })}
                      checked={selectedTeamMemberIdentityIDs.has(
                        member.identityId,
                      )}
                      onChange={(event) =>
                        toggleTeamMember(
                          member.identityId,
                          event.target.checked,
                        )
                      }
                    />
                  </TableCell>
                  <TableCell className="font-medium">
                    {member.displayName}
                  </TableCell>
                  <TableCell>
                    {t(
                      member.identityType ===
                        OrganizationIdentityType.OrganizationIdentityTypeAgent
                        ? "identityCategories.agent"
                        : "identityCategories.user",
                    )}
                  </TableCell>
                  <TableCell>
                    <WorkStatusBadge status={identityWorkStatus(member)} />
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-muted-foreground">
                    {formatDateTime(member.joinedAt)}
                  </TableCell>
                  <TableCell className="text-right whitespace-nowrap">
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
                          destructive
                          onSelect={() => setRemovingTeamMembers([member])}
                        >
                          {t("teams.members.remove")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
              {teamMembers.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell
                    colSpan={6}
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

      <ContactCreateDialogs
        scope="team"
        channels={channels}
        roles={roles}
        teams={teams}
        selectedTeam={selectedTeam}
        searchParams={searchParams}
        setParameters={setParameters}
      />

      {selectedTeam ? (
        <Dialog
          open={editingTeam}
          onOpenChange={(open) => !open && setParameters({ editTeam: null })}
        >
          <DialogContent className="max-w-xl">
            <DialogHeader>
              <DialogTitle>{t("teams.edit")}</DialogTitle>
              <DialogDescription>
                {t("teams.createDescription")}
              </DialogDescription>
            </DialogHeader>
            <TeamForm
              team={selectedTeam}
              onSaved={() => {
                void invalidate(resourceKeys.teams())
                invalidateMembershipCaches()
                setParameters({ editTeam: null })
              }}
              onCancel={() => setParameters({ editTeam: null })}
            />
          </DialogContent>
        </Dialog>
      ) : null}

      {selectedTeam ? (
        <Dialog
          open={addingTeamMembers}
          onOpenChange={(open) => !open && setParameters({ addMembers: null })}
        >
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>{t("teams.members.add")}</DialogTitle>
              <DialogDescription>
                {t("teams.members.addDescription", {
                  name: selectedTeam.name,
                })}
              </DialogDescription>
            </DialogHeader>
            <TeamMemberPicker
              team={selectedTeam}
              onSaved={() => {
                void invalidate(resourceKeys.teams())
                setParameters({ addMembers: null })
                void invalidate(resourceKeys.teamMembers(teamId))
                invalidateMembershipCaches()
              }}
              onCancel={() => setParameters({ addMembers: null })}
            />
          </DialogContent>
        </Dialog>
      ) : null}

      <AlertDialog
        open={deletingTeam !== null}
        onOpenChange={(open) => !open && setDeletingTeam(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("teams.delete.title", { name: deletingTeam?.name ?? "" })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("teams.delete.description", {
                count: deletingTeam?.memberCount ?? 0,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("teams.form.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void removeCurrentTeam()}
            >
              {t("teams.delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={removingTeamMembers.length > 0}
        onOpenChange={(open) => !open && setRemovingTeamMembers([])}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {removingTeamMembers.length === 1
                ? t("teams.members.removeTitle", {
                    name: removingTeamMembers[0].displayName,
                  })
                : t("teams.members.removeMultipleTitle", {
                    count: removingTeamMembers.length,
                  })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                removingTeamMembers.length === 1
                  ? "teams.members.removeDescription"
                  : "teams.members.removeMultipleDescription",
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("teams.form.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void removeMembersFromCurrentTeam()}
            >
              {t("teams.members.remove")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
