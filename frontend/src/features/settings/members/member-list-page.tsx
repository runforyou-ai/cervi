/** 设置中的成员账号管理页：新建、筛选、编辑资料与账号状态。 */
import { useCallback } from "react"
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  UserStatus,
  deactivateUser,
  listRoles,
  listTeams,
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
} from "@/components/list-toolbar"
import { PageHeader } from "@/components/page-header"
import { ResourceListLayout } from "@/components/resource-list"
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
import { useWorkspace } from "@/contexts/workspace-context"
import {
  AccountStatusFilter,
  useAccountStatusToggle,
} from "@/features/contacts/account-status-toggle"
import { contactResourceKeys, useContactInvalidator } from "@/features/contacts/use-contact-invalidator"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { MemberDetailSheet } from "@/features/settings/members/member-detail-sheet"
import { MemberForm } from "@/features/settings/members/member-form"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { roleDisplayName } from "@/lib/role-labels"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 加载并管理企业成员账号。 */
export function MemberListPage() {
  const { t } = useTranslation("contacts")
  const { t: tSettings } = useTranslation("settings")
  const { t: tCommon } = useTranslation("common")
  const { identity } = useWorkspace()
  const invalidateContact = useContactInvalidator()
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
  const creating = searchParams.get("new") === "1"
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
  const teamsResource = useResource(resourceKeys.teams({ pageSize: 100 }), () =>
    listTeams({ pageSize: 100 }),
  )
  const roles = rolesResource.data?.roles ?? []
  const teams = teamsResource.data?.teams ?? []
  const list = useResource(
    resourceKeys.users({ query, status, roleId, page: currentPage, pageSize: 50 }),
    () => listUsers({ query, status, roleId, page: currentPage, pageSize: 50 }),
  )
  const users = list.data?.users ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  // 保持引用稳定，供详情面板的读取失败处理依赖。
  const closeDetail = useCallback(
    () => setParameters({ selected: null }),
    [setParameters],
  )

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
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("add.member")}
          title={t("add.member")}
          onClick={() => setParameters({ new: "1" })}
        >
          <PlusIcon />
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
            setParameters({ roleId: value || null, page: null, selected: null })
          }
        />
        {hasFilters ? (
          <ListToolbarReset
            onClick={() => setParameters({ status: null, roleId: null, page: null })}
          >
            {tCommon("actions.clearFilters")}
          </ListToolbarReset>
        ) : null}
      </ListToolbar>

      <ResourceListLayout
        resources={list}
        errorMessage={t("list.loadError")}
        page={page}
        onPageChange={(number) =>
          setParameters({ page: String(number), selected: null })
        }
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
          ]}
          rows={users}
          rowKey={(user) => user.id}
          empty={t("list.empty")}
          onRowActivate={(user) => setParameters({ selected: user.id })}
          rowActions={(user) => [statusToggle.rowAction(user)]}
        />
      </ResourceListLayout>

      <MemberDetailSheet
        userId={selected}
        roles={roles}
        teams={teams}
        onClose={closeDetail}
      />

      <Dialog
        open={creating}
        onOpenChange={(open) => !open && setParameters({ new: null })}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("members.create")}</DialogTitle>
            <DialogDescription>{t("members.createDescription")}</DialogDescription>
          </DialogHeader>
          <MemberForm
            teams={teams}
            roles={roles}
            defaultTeamIds={[]}
            onSaved={() => {
              setParameters({ new: null })
              void invalidateContact("user")
            }}
            onCancel={() => setParameters({ new: null })}
          />
        </DialogContent>
      </Dialog>

      <ConfirmationDialog {...statusToggle.dialog} />
    </div>
  )
}
