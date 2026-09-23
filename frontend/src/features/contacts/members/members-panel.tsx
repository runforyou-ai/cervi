/** 企业成员列表、筛选、详情和账号状态管理面板。 */
import { useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  UserStatus,
  deactivateUser,
  listUsers,
  reactivateUser,
  type ChannelOption,
  type RoleData,
  type Team,
  type UserData,
} from "@/api"
import {
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { useWorkspace } from "@/contexts/workspace-context"
import {
  AccountStatusFilter,
  useAccountStatusToggle,
} from "@/features/contacts/account-status-toggle"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactListSection } from "@/features/contacts/contact-list-section"
import { MemberDetailSheet } from "@/features/contacts/members/member-detail-sheet"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { roleDisplayName } from "@/lib/role-labels"
import { contactResourceKeys } from "@/features/contacts/use-contact-invalidator"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
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
  const statusToggle = useAccountStatusToggle<UserData>({
    scope: "members",
    deactivate: deactivateUser,
    reactivate: reactivateUser,
    // 修改自己的账号状态时同时刷新当前身份。
    invalidateKeys: (user) => [
      ...contactResourceKeys("user", user.id),
      ...(user.id === identity.user.id ? [resourceKeys.identity()] : []),
    ],
    logLabel: "修改企业成员账号状态",
  })

  const list = useResource(
    resourceKeys.users({ query, status, roleId, page: currentPage, pageSize: 50 }),
    () => listUsers({ query, status, roleId, page: currentPage, pageSize: 50 }),
  )
  const users = list.data?.users ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  /** 返回成员行显示的工作状态。 */
  function memberWorkStatus(user: UserData) {
    return user.id === identity.user.id
      ? identity.user.workStatus
      : user.workStatus
  }

  // 详情关闭时一并清除新建参数；保持引用稳定，供详情面板的读取失败处理依赖。
  const closeDetail = useCallback(
    () => setParameters({ selected: null, new: null }),
    [setParameters],
  )

  const hasInternalFilters = Boolean(
    status !== UserStatus.UserStatusActive || roleId,
  )
  const roleOptions = roles.map((item) => ({
    value: item.id,
    label: roleDisplayName(item, tCommon),
  }))

  return (
    <>
      <ContactListSection
        title={t("scopes.employees")}
        description={t("scopeDescriptions.employees")}
        scope={{ scope: "employees", teams, channels }}
        toolbar={
          <>
            <ListToolbarSearch
              value={search}
              aria-label={t("search.employees")}
              onChange={(event) => setSearch(event.target.value)}
            />
            <AccountStatusFilter value={status} setParameters={setParameters} />
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
              key: "employee",
              header: t("columns.employeeName"),
              cellClassName: "min-w-0",
              cell: (user) => (
                <ResourceRowIdentity
                  avatar={{ imageURL: user.avatarUrl, name: user.displayName }}
                  status={memberWorkStatus(user)}
                  name={user.displayName}
                  secondary={roleDisplayName(user.role, tCommon)}
                  description={user.email}
                />
              ),
            },
          ]}
          rows={users}
          rowKey={(user) => user.id}
          empty={t("list.empty")}
          onRowActivate={(user) => setParameters({ selected: user.id })}
          // 自己没有发消息，已停用的成员保留禁用的发消息。
          rowActions={(user) => [
            ...(user.identityId !== identity.user.identityId
              ? [
                  {
                    key: "message",
                    label: t("sendMessage"),
                    disabled: user.status !== UserStatus.UserStatusActive,
                    onSelect: () =>
                      navigate(`/chats?target=${user.identityId}`),
                  },
                ]
              : []),
            statusToggle.rowAction(user),
          ]}
        />
      </ContactListSection>

      <MemberDetailSheet
        userId={selected}
        roles={roles}
        teams={teams}
        onClose={closeDetail}
      />

      <ContactCreateDialogs
        scope="employees"
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
