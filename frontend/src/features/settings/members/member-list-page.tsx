/** 设置中的成员账号列表：筛选、停用与恢复，并进入新建页和编辑页。 */
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useNavigate } from "react-router"

import {
  UserStatus,
  deactivateUser,
  listRoles,
  listUsers,
  reactivateUser,
  type UserData,
} from "@/api"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
  ListToolbarTotal,
} from "@/components/list-toolbar"
import { PageHeader } from "@/components/page-header"
import { ResourceListLayout } from "@/components/resource-list"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { Button } from "@/components/ui/button"
import { useWorkspace } from "@/contexts/workspace-context"
import {
  AccountStatusFilter,
  useAccountStatusToggle,
} from "@/features/contacts/account-status-toggle"
import { contactResourceKeys } from "@/features/contacts/use-contact-invalidator"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { usePagedResource, useResource } from "@/hooks/use-resource"
import { roleDisplayName } from "@/lib/role-labels"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 加载并管理企业成员账号。 */
export function MemberListPage() {
  const { t } = useTranslation("contacts")
  const { formatDateTime } = useDateTime()
  const { t: tSettings } = useTranslation("settings")
  const { t: tCommon } = useTranslation("common")
  const { identity } = useWorkspace()
  const navigate = useNavigate()
  const location = useLocation()
  const { searchParams, setParameters, query, search, setSearch } =
    useContactSearch()
  // 新建页和编辑页返回时恢复当前筛选和滚动位置。
  const returnQuery = new URLSearchParams({
    returnTo: location.pathname + location.search,
  }).toString()
  const status =
    optionalWailsEnum(UserStatus, searchParams.get("status")) ??
    UserStatus.UserStatusActive
  const roleId = searchParams.get("roleId") ?? ""
  const statusToggle = useAccountStatusToggle<UserData>({
    keyPrefix: "contacts:members.status",
    deactivate: deactivateUser,
    reactivate: reactivateUser,
    // 修改自己的账号状态时同时刷新当前身份。
    invalidateKeys: (user) => [
      ...contactResourceKeys("user", user.id),
      ...(user.id === identity.user.id ? [resourceKeys.identity()] : []),
    ],
    logLabel: "修改企业成员账号状态",
  })

  const rolesResource = useResource(resourceKeys.roles(), () => listRoles())
  const roles = rolesResource.data?.roles ?? []
  const list = usePagedResource(
    resourceKeys.users({ query, status, roleId, pageSize: 50 }),
    (page) => listUsers({ query, status, roleId, page, pageSize: 50 }),
    { select: (data) => ({ items: data.users, page: data.page }), itemKey: (user) => user.id },
  )
  const users = list.data?.items ?? []

  const hasFilters = Boolean(status !== UserStatus.UserStatusActive || roleId)
  const roleOptions = roles.map((item) => ({
    value: item.id,
    label: roleDisplayName(item, tCommon),
  }))

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={tSettings("members.title")}
        description={tSettings("members.description")}
      >
        <Button variant="ghost" size="icon-sm" asChild>
          <Link
            to={`/settings/members/new?${returnQuery}`}
            aria-label={t("add.member")}
            title={t("add.member")}
          >
            <PlusIcon />
          </Link>
        </Button>
      </PageHeader>

      <ListToolbar>
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
            setParameters({ roleId: value || null })
          }
        />
        {hasFilters ? (
          <ListToolbarReset
            onClick={() => setParameters({ status: null, roleId: null })}
          >
            {tCommon("actions.clearFilters")}
          </ListToolbarReset>
        ) : null}
        <ListToolbarTotal count={list.data?.total} />
      </ListToolbar>

      <ResourceListLayout
        resources={list}
        errorMessage={tSettings("members.loadError")}
        more={list.more}
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "member",
              header: t("columns.employeeName"),
              cellClassName: "min-w-0",
              cell: (user) => (
                <ResourceRowIdentity
                  avatar={{ imageURL: user.avatarUrl, name: user.displayName }}
                  name={user.displayName}
                  secondary={roleDisplayName(user.role, tCommon)}
                  description={user.email}
                />
              ),
            },
            {
              key: "time",
              header: t("columns.addedAt"),
              cellClassName: "w-px whitespace-nowrap text-right text-muted-foreground",
              cell: (user) =>
                t("list.addedAt", { time: formatDateTime(user.createdAt) }),
            },
          ]}
          rows={users}
          rowKey={(user) => user.id}
          empty={tSettings("members.empty")}
          onRowActivate={(user) => navigate(`/settings/members/${user.id}?${returnQuery}`)}
          rowActions={(user) => [statusToggle.rowAction(user)]}
        />
      </ResourceListLayout>

      <ConfirmationDialog {...statusToggle.dialog} />
    </div>
  )
}
