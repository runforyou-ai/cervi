/** 企业成员列表、筛选、详情和账号状态管理面板。 */
import { useEffect, useState } from "react"
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
import { ResourceTable } from "@/components/resource-table"
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
import { roleDisplayName } from "@/lib/role-labels"
import { useDateTime } from "@/hooks/use-date-time"
import { useContactInvalidator } from "@/features/contacts/use-contact-invalidator"
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
  const { identity } = useWorkspace()
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const invalidateContact = useContactInvalidator()
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

  /** 返回成员行显示的工作状态。 */
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
    void invalidateContact("user")
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
      void invalidateContact("user", saved.id)
      if (saved.id === identity.user.id) void invalidate(resourceKeys.identity())
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
          <ResourceTable
            columns={[
              {
                key: "employeeName",
                header: t("columns.employeeName"),
                cellClassName: "font-medium",
                cell: (user) => user.displayName,
              },
              {
                key: "email",
                header: t("columns.email"),
                cellClassName: "text-muted-foreground",
                cell: (user) => user.email,
              },
              {
                key: "joinedTeams",
                header: t("columns.joinedTeams"),
                cellClassName: "max-w-xs",
                cell: (user) => <JoinedTeamsCell teams={user.teams} />,
              },
              {
                key: "role",
                header: t("columns.role"),
                cell: (user) => roleDisplayName(user.role, tCommon),
              },
              {
                key: "accountStatus",
                header: t("columns.accountStatus"),
                cell: (user) => (
                  <UserStatusBadge
                    status={user.status}
                    label={userStatusLabel(user.status, t)}
                  />
                ),
              },
              {
                key: "workStatus",
                header: t("columns.workStatus"),
                cell: (user) => (
                  <WorkStatusBadge status={memberWorkStatus(user)} />
                ),
              },
              {
                key: "createdAt",
                header: t("columns.createdAt"),
                cellClassName: "whitespace-nowrap text-muted-foreground",
                cell: (user) => formatDateTime(user.createdAt),
              },
            ]}
            rows={users}
            rowKey={(user) => user.id}
            empty={t("list.empty")}
            actions={(user) => ({
              primary: (
                <>
                  <Button
                    size="sm"
                    disabled={
                      user.status !== UserStatus.UserStatusActive ||
                      user.identityId === identity.user.identityId
                    }
                    onClick={() =>
                      navigate(`/inbox?scope=internal&target=${user.identityId}`)
                    }
                  >
                    {t("sendMessage")}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setParameters({ selected: user.id })}
                  >
                    {tCommon("actions.view")}
                  </Button>
                </>
              ),
              menu: (
                <DropdownMenuItem
                  destructive={user.status === UserStatus.UserStatusActive}
                  onSelect={() => setChangingUserStatus(user)}
                >
                  {t(
                    user.status === UserStatus.UserStatusActive
                      ? "members.status.deactivate"
                      : "members.status.reactivate",
                  )}
                </DropdownMenuItem>
              ),
            })}
          />
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
              void invalidateContact("user", saved.id)
              if (saved.id === identity.user.id) {
                void invalidate(resourceKeys.identity())
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
            <AlertDialogCancel>{tCommon("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void changeUserStatus()}
            >
              {deleting
                ? t("members.status.saving")
                : tCommon("actions.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
