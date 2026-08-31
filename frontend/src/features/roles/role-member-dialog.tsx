/** 角色成员的左右分栏配置对话框。 */
import { useEffect, useMemo, useState } from "react"
import { SearchIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  listAgents,
  listUsers,
  RoleKind,
  type AgentListItemData,
  type UserData,
  type RoleData,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { roleDisplayName } from "@/features/roles/role-labels"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

const memberPageSize = 100

/** 描述待随角色表单保存的成员调整。 */
export type RoleMemberChange = {
  member: RoleMemberData
  previousRoleID: string
  nextRoleID: string
}

type RoleMemberOption = Pick<RoleData, "id" | "kind" | "name">
type RoleMemberData = UserData | AgentListItemData

/** 读取当前企业的全部成员。 */
async function listAllMembers() {
  const users: UserData[] = []
  const agents: AgentListItemData[] = []
  let page = 1
  let pages = 1
  do {
    const output = await listUsers({ page, pageSize: memberPageSize })
    users.push(...output.users)
    pages = Math.ceil(output.page.total / memberPageSize)
    page += 1
  } while (page <= pages)
  page = 1
  pages = 1
  do {
    const output = await listAgents({ page, pageSize: memberPageSize })
    agents.push(...output.agents)
    pages = Math.ceil(output.page.total / memberPageSize)
    page += 1
  } while (page <= pages)
  return [...users, ...agents]
}

/** 判断成员姓名或邮箱是否包含搜索内容。 */
function matchesMember(user: RoleMemberData, query: string) {
  const keyword = query.trim().toLocaleLowerCase()
  if (!keyword) return true
  return `${user.displayName}\n${"email" in user ? user.email : ""}`
    .toLocaleLowerCase()
    .includes(keyword)
}

/** 显示一名可添加或已添加的企业成员。 */
function MemberRow({
  user,
  assignedRoleName,
  action,
  actionLabel,
  disabled = false,
  actionDisabled = false,
}: {
  user: RoleMemberData
  assignedRoleName?: string
  action: () => void
  actionLabel: string
  disabled?: boolean
  actionDisabled?: boolean
}) {
  const { t } = useTranslation("settings")
  return (
    <li
      className={cn(
        "flex min-h-16 items-center gap-3 border-b px-3 py-2 last:border-b-0",
        disabled && "bg-muted/40 text-muted-foreground",
      )}
    >
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{user.displayName}</p>
        <p className="truncate text-xs text-muted-foreground">
          {"email" in user ? user.email : t("roles.members.aiEmployee")}
        </p>
        {assignedRoleName ? (
          <p className="mt-1 truncate text-xs text-muted-foreground">
            {t("roles.members.assignedTo", { role: assignedRoleName })}
          </p>
        ) : null}
      </div>
      <Button
        type="button"
        size="xs"
        variant="outline"
        disabled={actionDisabled}
        onClick={action}
      >
        {actionLabel}
      </Button>
    </li>
  )
}

/** 管理一个角色包含的企业成员。 */
export function RoleMemberDialog({
  role,
  roles,
  pendingRoleIDs,
  onOpenChange,
  onConfirm,
}: {
  role: RoleMemberOption | null
  roles: RoleMemberOption[]
  pendingRoleIDs: Record<string, string>
  onOpenChange: (open: boolean) => void
  onConfirm: (changes: RoleMemberChange[]) => void
}) {
  const { t } = useTranslation("settings")
  const { t: tCommon } = useTranslation("common")
  const [query, setQuery] = useState("")
  const [draftRoleIDs, setDraftRoleIDs] = useState<Record<string, string>>({})
  const defaultRole = roles.find(
    (item) => item.kind === RoleKind.RoleKindMember,
  )
  const {
    data: loadedMembers,
    loading: membersLoading,
    error: membersError,
  } = useResource(resourceKeys.roleMembers(), () => listAllMembers(), {
    enabled: role !== null,
  })
  const members = useMemo(() => loadedMembers ?? [], [loadedMembers])
  const loading = role !== null && membersLoading
  const error = role !== null && Boolean(membersError)

  /** 打开对话框时按最新成员和暂存调整初始化草稿。 */
  useEffect(() => {
    if (!role) {
      setQuery("")
      setDraftRoleIDs({})
      return
    }
    if (!loadedMembers) return
    const roleIDs = Object.fromEntries(
      loadedMembers.map((member) => [member.identityId, member.role.id]),
    )
    setDraftRoleIDs({ ...roleIDs, ...pendingRoleIDs })
  }, [loadedMembers, pendingRoleIDs, role])

  const selectedUsers = role
    ? members.filter(
        (member) =>
          (draftRoleIDs[member.identityId] ?? member.role.id) === role.id,
      )
    : []
  const availableUsers = useMemo(
    () =>
      role
        ? members.filter(
            (member) =>
              !(
                role.kind === RoleKind.RoleKindAdmin &&
                !("email" in member)
              ) &&
              (draftRoleIDs[member.identityId] ?? member.role.id) !== role.id &&
              matchesMember(member, query),
          )
        : [],
    [draftRoleIDs, members, query, role],
  )
  const changedUsers = members.filter(
    (member) =>
      (draftRoleIDs[member.identityId] ?? member.role.id) !== member.role.id,
  )

  /** 暂存成员的目标角色。 */
  function stageUserRole(user: RoleMemberData, nextRole: RoleMemberOption) {
    setDraftRoleIDs((current) => ({
      ...current,
      [user.identityId]: nextRole.id,
    }))
  }

  /** 确认成员调整并交给角色详情页暂存。 */
  function confirmChanges() {
    onConfirm(
      changedUsers.map((user) => ({
        member: user,
        previousRoleID: user.role.id,
        nextRoleID: draftRoleIDs[user.identityId] ?? user.role.id,
      })),
    )
    onOpenChange(false)
  }

  const roleName = role ? roleDisplayName(role, tCommon) : ""
  const emptyText = query
    ? t("roles.members.noSearchResults")
    : t("roles.members.emptyAvailable")

  return (
    <Dialog open={role !== null} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-w-4xl overflow-hidden"
        aria-describedby={undefined}
      >
        <DialogHeader>
          <DialogTitle>
            {t("roles.members.title", { role: roleName })}
          </DialogTitle>
        </DialogHeader>
        {loading ? (
          <LoadingIndicator className="h-80 justify-center rounded-lg border">
            {t("roles.members.loading")}
          </LoadingIndicator>
        ) : error ? (
          <div className="flex h-80 items-center justify-center rounded-lg border text-sm text-muted-foreground">
            {t("roles.members.loadError")}
          </div>
        ) : (
          <div className="grid min-h-0 overflow-hidden rounded-lg border md:grid-cols-2">
            <section className="min-w-0 border-b md:border-r md:border-b-0">
              <div className="flex h-14 items-center border-b bg-muted/30 px-3 py-2">
                <div className="relative w-full">
                  <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    type="search"
                    className="pl-9"
                    aria-label={t("roles.members.searchAria")}
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                  />
                </div>
              </div>
              <ScrollArea className="h-72">
                {availableUsers.length === 0 ? (
                  <p className="flex h-24 items-center justify-center px-4 text-center text-sm text-muted-foreground">
                    {emptyText}
                  </p>
                ) : (
                  <ul>
                    {availableUsers.map((user) => {
                      const draftRoleID =
                        draftRoleIDs[user.identityId] ?? user.role.id
                      const draftRole =
                        roles.find((item) => item.id === draftRoleID) ?? user.role
                      const assigned = draftRole.kind !== RoleKind.RoleKindMember
                      const assignedRoleName = roleDisplayName(draftRole, tCommon)
                      return (
                        <MemberRow
                          key={user.identityId}
                          user={user}
                          disabled={assigned}
                          assignedRoleName={assigned ? assignedRoleName : undefined}
                          action={() => {
                            if (role) stageUserRole(user, role)
                          }}
                          actionLabel={t("roles.members.add")}
                        />
                      )
                    })}
                  </ul>
                )}
              </ScrollArea>
            </section>
            <section className="min-w-0">
              <div className="flex h-14 items-center border-b bg-muted/30 px-3 py-2">
                <h3 className="text-sm font-medium">
                  {t("roles.members.selected", {
                    count: selectedUsers.length,
                  })}
                </h3>
              </div>
              <ScrollArea className="h-72">
                {selectedUsers.length === 0 ? (
                  <p className="flex h-24 items-center justify-center px-4 text-center text-sm text-muted-foreground">
                    {t("roles.members.emptySelected")}
                  </p>
                ) : (
                  <ul>
                    {selectedUsers.map((user) => (
                      <MemberRow
                        key={user.identityId}
                        user={user}
                        disabled={!defaultRole || role?.id === defaultRole.id}
                        actionDisabled={!defaultRole || role?.id === defaultRole.id}
                        action={() => {
                          if (defaultRole) stageUserRole(user, defaultRole)
                        }}
                        actionLabel={t("roles.members.remove")}
                      />
                    ))}
                  </ul>
                )}
              </ScrollArea>
            </section>
          </div>
        )}
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("roles.members.cancel")}
          </Button>
          <Button
            type="button"
            disabled={loading || error}
            onClick={confirmChanges}
          >
            {t("roles.members.confirm")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
