/** 企业成员列表、筛选、详情和账号状态管理面板。 */
import { useEffect, useState } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  UserStatus,
  deactivateUser,
  getUser,
  isApiError,
  listUsers,
  reactivateUser,
  sessionPath,
  type ChannelOption,
  type RoleData,
  type Team,
  type UserData,
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
import { useWorkspace } from "@/contexts/workspace-context"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactDetailSheet } from "@/features/contacts/contact-detail-sheet"
import { ContactListLayout } from "@/features/contacts/contact-list-layout"
import { ContactScopeMobileSelect } from "@/features/contacts/contact-scope-mobile-select"
import { userStatusLabel } from "@/features/contacts/external/contact-labels"
import { JoinedTeamsCell } from "@/features/contacts/joined-teams-cell"
import { MemberDetailView } from "@/features/contacts/members/member-detail"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { UserStatusBadge } from "@/features/contacts/user-status-badge"
import { roleDisplayName } from "@/features/roles/role-labels"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 企业成员范围的列表、详情和弹窗。 */
export function MembersPanel({
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
  const { identity, updateUser: updateWorkspaceUser } = useWorkspace()
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
  const roleId = searchParams.get("roleId") ?? ""
  const [changingUserStatus, setChangingUserStatus] =
    useState<UserData | null>(null)
  const [deleting, setDeleting] = useState(false)

  const list = useResource(
    resourceKeys.users({ query, status, roleId, page: currentPage, pageSize: 50 }),
    () => listUsers({ query, status, roleId, page: currentPage, pageSize: 50 }),
  )
  const users = list.data?.users ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  const detail = useResource(resourceKeys.user(selected), () => getUser(selected), {
    enabled: Boolean(selected),
  })
  const detailUser = selected ? (detail.data ?? null) : null

  const detailError = detail.error
  useEffect(() => {
    if (!selected || !detailError) return
    if (isApiError(detailError) && sessionPath(detailError.state)) return
    console.warn("联系人详情加载失败", detailError)
    toast.error(t("detail.loadError"))
    setParameters({ selected: null })
  }, [detailError, selected, setParameters, t])

  /** 当前用户使用工作台中的即时状态，其他成员使用目录查询结果。 */
  function memberWorkStatus(user: UserData) {
    return user.id === identity.user.id
      ? identity.user.workStatus
      : user.workStatus
  }

  /** 关闭成员详情。 */
  function closeDetail() {
    setParameters({ selected: null, new: null })
  }

  /** 刷新列表并关闭详情。 */
  function refreshAndClose() {
    closeDetail()
    void invalidate(resourceKeys.users())
  }

  /** 禁用用户账号或恢复为正常状态。 */
  async function changeUserStatus() {
    if (!changingUserStatus) return
    setDeleting(true)
    try {
      const saved =
        changingUserStatus.status === UserStatus.UserStatusActive
          ? await deactivateUser(changingUserStatus.id)
          : await reactivateUser(changingUserStatus.id)
      console.info("企业成员账号状态已修改", {
        identity_id: saved.identityId,
        user_id: saved.id,
        status: saved.status,
      })
      toast.success(
        t(
          changingUserStatus.status === UserStatus.UserStatusActive
            ? "members.status.deactivated"
            : "members.status.reactivated",
        ),
      )
      setChangingUserStatus(null)
      void invalidate(resourceKeys.user(saved.id))
      void invalidate(resourceKeys.users())
      void invalidate(resourceKeys.customerServiceAssignees())
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("修改企业成员账号状态失败", {
        user_id: changingUserStatus.id,
        error,
      })
      toast.error(t("members.status.error"))
    } finally {
      setDeleting(false)
    }
  }

  const hasInternalFilters = Boolean(
    status !== UserStatus.UserStatusActive || roleId,
  )
  const roleOptions = roles.map((item) => ({
    value: item.id,
    label: roleDisplayName(item, tCommon),
  }))

  return (
    <>
      <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <PageHeader
          title={t("scopes.employees")}
          beforeTitle={
            <ContactScopeMobileSelect
              scope="employees"
              teams={teams}
              channels={channels}
            />
          }
        />

        <ListToolbar>
          <ListToolbarSearch
            value={search}
            aria-label={t("search.employees")}
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
          <ListToolbarFilter
            label={t("filters.role")}
            allLabel={t("filters.allRoles")}
            value={roleId}
            options={roleOptions}
            contentClassName="max-h-[min(18rem,var(--radix-dropdown-menu-content-available-height))]"
            onValueChange={(value) =>
              setParameters({
                roleId: value || null,
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
                <TableHead>{t("columns.employeeName")}</TableHead>
                <TableHead>{t("columns.email")}</TableHead>
                <TableHead>{t("columns.joinedTeams")}</TableHead>
                <TableHead>{t("columns.role")}</TableHead>
                <TableHead>{t("columns.accountStatus")}</TableHead>
                <TableHead>{t("columns.workStatus")}</TableHead>
                <TableHead>{t("columns.createdAt")}</TableHead>
                <TableHead className="text-right">
                  {t("columns.actions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium">
                    {user.displayName}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {user.email}
                  </TableCell>
                  <TableCell className="max-w-xs">
                    <JoinedTeamsCell teams={user.teams} />
                  </TableCell>
                  <TableCell>{roleDisplayName(user.role, tCommon)}</TableCell>
                  <TableCell>
                    <UserStatusBadge
                      status={user.status}
                      label={userStatusLabel(user.status, t)}
                    />
                  </TableCell>
                  <TableCell>
                    <WorkStatusBadge status={memberWorkStatus(user)} />
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-muted-foreground">
                    {formatDateTime(user.createdAt)}
                  </TableCell>
                  <TableCell className="text-right whitespace-nowrap">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setParameters({ selected: user.id })}
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
                              user.status === UserStatus.UserStatusActive
                            }
                            onSelect={() => setChangingUserStatus(user)}
                          >
                            {t(
                              user.status === UserStatus.UserStatusActive
                                ? "members.status.deactivate"
                                : "members.status.reactivate",
                            )}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {users.length === 0 ? (
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
        title={detailUser?.displayName ?? t("detail.memberTitle")}
        description={t("detail.memberDescription")}
        loading={detail.loading && Boolean(selected)}
      >
        {detailUser ? (
          <MemberDetailView
            key={detailUser.id}
            user={detailUser}
            teams={teams}
            roles={roles}
            workStatus={memberWorkStatus(detailUser)}
            onSaved={(saved) => {
              void invalidate(resourceKeys.user(saved.id))
              void invalidate(resourceKeys.users())
              void invalidate(resourceKeys.roles())
              void invalidate(resourceKeys.teams())
              void invalidate(resourceKeys.teamMembers())
              void invalidate(resourceKeys.teamMemberCandidates())
              void invalidate(resourceKeys.roleMembers())
              void invalidate(resourceKeys.customerServiceAssignees())
              if (saved.id === identity.user.id) {
                updateWorkspaceUser({
                  ...identity.user,
                  displayName: saved.displayName,
                  email: saved.email,
                  roleId: saved.role.id,
                  status: saved.status,
                })
              }
            }}
            onNotFound={refreshAndClose}
          />
        ) : null}
      </ContactDetailSheet>

      <ContactCreateDialogs
        scope="employees"
        channels={channels}
        roles={roles}
        teams={teams}
        searchParams={searchParams}
        setParameters={setParameters}
      />

      <AlertDialog
        open={changingUserStatus !== null}
        onOpenChange={(open) => !open && setChangingUserStatus(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(
                changingUserStatus?.status === UserStatus.UserStatusActive
                  ? "members.status.deactivateTitle"
                  : "members.status.reactivateTitle",
                { name: changingUserStatus?.displayName ?? "" },
              )}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                changingUserStatus?.status === UserStatus.UserStatusActive
                  ? "members.status.deactivateDescription"
                  : "members.status.reactivateDescription",
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("members.status.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void changeUserStatus()}
            >
              {deleting
                ? t("members.status.saving")
                : t("members.status.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
