/** 角色与权限列表页。 */
import { useEffect, useRef, useState } from "react"
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
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import {
  ResourceTable,
  ResourceTableActions,
} from "@/components/resource-table"
import { SelectableText } from "@/components/selectable-text"
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
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { TableCell } from "@/components/ui/table"
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
      <PageHeader title={t("roles.title")}>
        {maximum !== null && roles.length < maximum ? (
          <Button size="sm" asChild>
            <Link to="/settings/roles/new">{t("roles.list.create")}</Link>
          </Button>
        ) : (
          <Button
            size="sm"
            disabled
            title={maximum === null ? undefined : t("roles.list.limitReached")}
          >
            {t("roles.list.create")}
          </Button>
        )}
      </PageHeader>
      <PageContent>
        {showLoading ? (
          <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
            {tCommon("status.loading")}
          </LoadingIndicator>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("roles.list.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refresh()}
            >
              {tCommon("actions.retry")}
            </Button>
          </div>
        ) : (
          <div className="@container overflow-hidden rounded-lg border bg-card">
            <ResourceTable
              columns={[
                { key: "name", header: t("roles.list.columns.name") },
                {
                  key: "description",
                  header: t("roles.list.columns.description"),
                  className: "hidden w-64 @3xl:table-cell",
                },
                {
                  key: "memberCount",
                  header: t("roles.list.columns.memberCount"),
                },
                {
                  key: "permissions",
                  header: t("roles.list.columns.permissions"),
                  className: "hidden @3xl:table-cell",
                },
                {
                  key: "actions",
                  header: tCommon("table.actions"),
                  className: "w-px",
                },
              ]}
              rows={roles}
              rowKey={(role) => role.id}
              empty={t("roles.list.empty")}
            >
              {(role) => {
                const description = roleDescription(role, t)
                return (
                  <>
                    <TableCell className="font-medium">
                      <SelectableText>
                        {roleDisplayName(role, tCommon)}
                      </SelectableText>
                    </TableCell>
                    <TableCell className="hidden max-w-64 text-muted-foreground @3xl:table-cell">
                      <span className="block truncate" title={description}>
                        {description}
                      </span>
                    </TableCell>
                    <TableCell>{role.memberCount}</TableCell>
                    <TableCell className="hidden text-muted-foreground @3xl:table-cell">
                      {permissionSummary(role, permissions, t)}
                    </TableCell>
                    <ResourceTableActions
                      menu={
                        <DropdownMenuItem
                          destructive
                          disabled={role.kind !== RoleKind.RoleKindCustom}
                          onSelect={() => setDeletingRole(role)}
                        >
                          {tCommon("actions.delete")}
                        </DropdownMenuItem>
                      }
                    >
                      <Button variant="outline" size="sm" asChild>
                        <Link to={`/settings/roles/${role.id}`}>
                          {tCommon("actions.view")}
                        </Link>
                      </Button>
                    </ResourceTableActions>
                  </>
                )
              }}
            </ResourceTable>
          </div>
        )}
      </PageContent>

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
              {deleting ? tCommon("actions.deleting") : tCommon("actions.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
