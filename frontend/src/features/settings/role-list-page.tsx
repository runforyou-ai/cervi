/** 角色与权限列表页。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteRole,
  isApiError,
  listRoles,
  PermissionLevel,
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
import { roleDisplayName } from "@/features/roles/role-labels"
import { recoverSession } from "@/lib/session-navigation"
import { apiErrorMessage } from "@/lib/form-errors"

/** 显示角色已配置的查看和管理权限数量。 */
function permissionSummary(
  role: RoleData,
  definitions: PermissionDefinition[],
  t: ReturnType<typeof useTranslation<"settings">>["t"],
) {
  const selected = new Set(role.permissions)
  const view = definitions.filter(
    (item) =>
      item.level === PermissionLevel.PermissionLevelView &&
      selected.has(item.code),
  ).length
  const manage = definitions.filter(
    (item) =>
      item.level === PermissionLevel.PermissionLevelManage &&
      selected.has(item.code),
  ).length
  return t("roles.list.permissionSummary", { view, manage })
}

/** 加载并管理企业角色列表。 */
export function RoleListPage() {
  const { t } = useTranslation("settings")
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const [roles, setRoles] = useState<RoleData[]>([])
  const [permissions, setPermissions] = useState<PermissionDefinition[]>([])
  const [maximum, setMaximum] = useState<number | null>(null)
  const [deletingRole, setDeletingRole] = useState<RoleData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const loadVersion = useRef(0)
  const mounted = useRef(true)

  /** 读取角色和权限目录。 */
  const load = useCallback(async () => {
    const version = ++loadVersion.current
    setLoading(true)
    setError(false)
    try {
      const output = await listRoles()
      if (version !== loadVersion.current) return
      setRoles(output.roles)
      setPermissions(output.permissions)
      setMaximum(output.maximum)
    } catch (requestError) {
      if (version !== loadVersion.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("角色列表加载失败", requestError)
      setError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [navigate])

  useEffect(() => {
    mounted.current = true
    void load()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [load])

  /** 删除选中的自定义角色。 */
  async function confirmDelete() {
    if (!deletingRole || deleting) return
    setDeleting(true)
    try {
      await deleteRole(deletingRole.id)
      if (!mounted.current) return
      setRoles((current) =>
        current.filter((role) => role.id !== deletingRole.id),
      )
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
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("roles.loading")}
          </div>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("roles.list.loadError")}
            </p>
            <Button className="mt-4" variant="outline" onClick={load}>
              {t("roles.retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("roles.list.columns.name")}</TableHead>
                  <TableHead>{t("roles.list.columns.memberCount")}</TableHead>
                  <TableHead>{t("roles.list.columns.permissions")}</TableHead>
                  <TableHead className="text-right">
                    {t("roles.list.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {roles.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={4}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {t("roles.list.empty")}
                    </TableCell>
                  </TableRow>
                ) : roles.map((role) => (
                  <TableRow key={role.id}>
                    <TableCell className="font-medium">
                      <SelectableText>
                        {roleDisplayName(role, tCommon)}
                      </SelectableText>
                    </TableCell>
                    <TableCell>{role.memberCount}</TableCell>
                    <TableCell className="text-muted-foreground">
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
                ))}
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
