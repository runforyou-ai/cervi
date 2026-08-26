/** 角色与权限列表页。 */
import { useEffect, useRef, useState } from "react"
import { LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"
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
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
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
import {
  permissionDefinitionLabel,
  roleDescription,
  roleDisplayName,
} from "@/features/roles/role-labels"
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
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("roles.loading")}
          </div>
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
              {t("roles.retry")}
            </Button>
          </div>
        ) : (
          <div className="@container overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("roles.list.columns.name")}</TableHead>
                  <TableHead className="hidden w-64 @3xl:table-cell">
                    {t("roles.list.columns.description")}
                  </TableHead>
                  <TableHead>{t("roles.list.columns.memberCount")}</TableHead>
                  <TableHead className="hidden @3xl:table-cell">
                    {t("roles.list.columns.permissions")}
                  </TableHead>
                  <TableHead className="text-right">
                    {t("roles.list.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {roles.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={5}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {t("roles.list.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  roles.map((role) => {
                    const description = roleDescription(role, t)
                    return (
                      <TableRow key={role.id}>
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
                        <TableCell className="text-right whitespace-nowrap">
                          <div className="flex justify-end gap-2">
                            <Button variant="outline" size="sm" asChild>
                              <Link to={`/settings/roles/${role.id}`}>
                                {t("roles.list.view")}
                              </Link>
                            </Button>
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  aria-label={t("roles.list.more")}
                                  title={t("roles.list.more")}
                                >
                                  <MoreHorizontalIcon />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuItem
                                  disabled={role.kind !== RoleKind.RoleKindCustom}
                                  className="text-destructive focus:text-destructive"
                                  onSelect={() => setDeletingRole(role)}
                                >
                                  {t("roles.list.delete")}
                                </DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
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
              {t("roles.delete.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void confirmDelete()}
            >
              {deleting ? t("roles.delete.deleting") : t("roles.delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
