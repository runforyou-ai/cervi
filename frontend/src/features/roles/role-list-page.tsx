/** 角色与权限列表页。 */
import { useEffect, useRef, useState } from "react"
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteRole,
  isApiError,
  listRoles,
  RoleKind,
  type PermissionDefinition,
  type RoleData,
} from "@/api"
import { ListActionButton } from "@/components/list-action-button"
import { PageHeader } from "@/components/page-header"
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
import { Button } from "@/components/ui/button"
import {
  permissionDefinitionLabel,
  roleDescription,
  roleDisplayName,
} from "@/lib/role-labels"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { apiErrorMessage } from "@/lib/form-errors"

/** 显示角色已配置的权限名称和总数。 */
function permissionSummary(
  role: RoleData,
  definitions: PermissionDefinition[],
  t: ReturnType<typeof useTranslation<"settings">>["t"],
) {
  const selected = new Set(role.permissions)
  const labels = definitions
    .filter((item) => selected.has(item.code))
    .map((item) => permissionDefinitionLabel(item, t))
  if (labels.length === 0) return t("roles.list.permissionEmpty")
  const items = labels.slice(0, 2).join(t("roles.list.permissionSeparator"))
  return labels.length > 2
    ? t("roles.list.permissionSummary", { items, count: labels.length })
    : items
}

/** 加载并管理企业角色列表。 */
export function RoleListPage() {
  const { t } = useTranslation("settings")
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const [deletingRole, setDeletingRole] = useState<RoleData | null>(null)
  const [deleting, setDeleting] = useState(false)
  const mounted = useRef(true)
  const invalidate = useResourceInvalidator()
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.roles(),
    () => listRoles(),
  )
  const showLoading = loading || (Boolean(error) && refreshing)
  const roles = data?.roles ?? []
  const permissions = data?.permissions ?? []
  const maximum = data?.maximum ?? null

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 删除选中的自定义角色。 */
  async function confirmDelete() {
    if (!deletingRole || deleting) return
    setDeleting(true)
    try {
      await deleteRole(deletingRole.id)
      if (!mounted.current) return
      void refresh()
      void invalidate(resourceKeys.role(deletingRole.id))
      setDeletingRole(null)
      toast.success(t("roles.delete.success"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("删除角色失败", {
        role_id: deletingRole.id,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError)
          : t("roles.delete.error"),
      )
    } finally {
      if (mounted.current) setDeleting(false)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={t("roles.title")}
        description={t("roles.description")}
      >
        {maximum !== null && roles.length < maximum ? (
          <Button variant="ghost" size="icon-sm" asChild>
            <Link
              to="/settings/roles/new"
              aria-label={t("roles.list.create")}
              title={t("roles.list.create")}
            >
              <PlusIcon />
            </Link>
          </Button>
        ) : (
          <Button
            variant="ghost"
            size="icon-sm"
            disabled
            aria-label={t("roles.list.create")}
            title={
              maximum === null
                ? t("roles.list.create")
                : t("roles.list.limitReached")
            }
          >
            <PlusIcon />
          </Button>
        )}
      </PageHeader>
      <ResourceListLayout
        loading={showLoading}
        error={Boolean(error)}
        errorMessage={t("roles.list.loadError")}
        onRetry={() => void refresh()}
        frameClassName="@container"
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "role",
              header: t("roles.list.columns.name"),
              cell: (role) => {
                const description = roleDescription(role, t)
                return (
                  <div className="min-w-0">
                    <p className="truncate font-medium">
                      {roleDisplayName(role, tCommon)}
                    </p>
                    <p className="truncate text-xs text-muted-foreground">
                      {t("roles.list.memberCount", { count: role.memberCount })}
                      <span aria-hidden="true"> · </span>
                      {description}
                    </p>
                  </div>
                )
              },
            },
            {
              key: "permissions",
              header: t("roles.list.columns.permissions"),
              className: "hidden @3xl:table-cell",
              cellClassName: "text-muted-foreground",
              cell: (role) => permissionSummary(role, permissions, t),
            },
          ]}
          rows={roles}
          rowKey={(role) => role.id}
          empty={t("roles.list.empty")}
          onRowActivate={(role) => navigate(`/settings/roles/${role.id}`)}
          actions={(role) => ({
            primary:
              role.kind === RoleKind.RoleKindCustom ? (
                <ListActionButton
                  tone="destructive"
                  onClick={() => setDeletingRole(role)}
                >
                  {tCommon("actions.delete")}
                </ListActionButton>
              ) : null,
          })}
        />
      </ResourceListLayout>

      <AlertDialog
        open={deletingRole !== null}
        onOpenChange={(open) => !open && !deleting && setDeletingRole(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {deletingRole
                ? t("roles.delete.title", {
                    name: roleDisplayName(deletingRole, tCommon),
                  })
                : null}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("roles.delete.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {tCommon("actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void confirmDelete()}
            >
              {deleting ? tCommon("actions.deleting") : tCommon("actions.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
