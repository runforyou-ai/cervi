/** 业务系统列表页。 */
import { useEffect, useRef, useState } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteBusinessSystem,
  isApiError,
  listBusinessSystems,
  type BusinessSystem,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { SelectableText } from "@/components/selectable-text"
import { StatusBadge } from "@/components/status-badge"
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
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 显示当前企业配置的业务系统。 */
export function BusinessSystemListPage() {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const [deletingBusinessSystem, setDeletingBusinessSystem] =
    useState<BusinessSystem | null>(null)
  const [deleting, setDeleting] = useState(false)
  const mounted = useRef(true)
  const {
    data,
    loading,
    refreshing,
    error: loadError,
    refresh,
  } = useResource(resourceKeys.businessSystems(), () => listBusinessSystems())
  const invalidate = useResourceInvalidator()
  const showLoading = loading || (Boolean(loadError) && refreshing)
  const businessSystems = data?.businessSystems ?? []

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 删除选中的业务系统。 */
  async function confirmDelete() {
    if (!deletingBusinessSystem || deleting) return
    setDeleting(true)
    try {
      await deleteBusinessSystem(deletingBusinessSystem.id)
      if (!mounted.current) return
      void refresh()
      void invalidate(resourceKeys.businessSystem(deletingBusinessSystem.id))
      console.info("业务系统已删除", {
        business_system_id: deletingBusinessSystem.id,
      })
      setDeletingBusinessSystem(null)
      toast.success(t("businessSystem.delete.success"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("业务系统删除失败", {
        business_system_id: deletingBusinessSystem.id,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError)
          : t("businessSystem.delete.error"),
      )
    } finally {
      if (mounted.current) setDeleting(false)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("businessSystem.title")}>
        <Button size="sm" asChild>
          <Link to="/integrations/business-systems/new">
            {t("businessSystem.list.create")}
          </Link>
        </Button>
      </PageHeader>
      <PageContent>
        {showLoading ? (
          <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
            {t("businessSystem.loading")}
          </LoadingIndicator>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("businessSystem.list.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refresh()}
            >
              {t("businessSystem.retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("businessSystem.list.columns.name")}</TableHead>
                  <TableHead>{t("businessSystem.list.columns.url")}</TableHead>
                  <TableHead>{t("businessSystem.list.columns.status")}</TableHead>
                  <TableHead className="text-right">
                    {t("businessSystem.list.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {businessSystems.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={4}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {t("businessSystem.list.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  businessSystems.map((businessSystem) => (
                    <TableRow key={businessSystem.id}>
                      <TableCell className="font-medium">
                        <SelectableText>{businessSystem.name}</SelectableText>
                      </TableCell>
                      <TableCell className="max-w-xl text-muted-foreground">
                        <SelectableText>{businessSystem.url}</SelectableText>
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          variant={businessSystem.enabled ? "success" : "muted"}
                          showDot={false}
                        >
                          {businessSystem.enabled
                            ? t("businessSystem.status.enabled")
                            : t("businessSystem.status.disabled")}
                        </StatusBadge>
                      </TableCell>
                      <TableCell className="text-right whitespace-nowrap">
                        <div className="flex justify-end gap-2">
                          <Button variant="outline" size="sm" asChild>
                            <Link
                              to={`/integrations/business-systems/${businessSystem.id}`}
                            >
                              {t("businessSystem.list.edit")}
                            </Link>
                          </Button>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                aria-label={t("businessSystem.list.more")}
                                title={t("businessSystem.list.more")}
                              >
                                <MoreHorizontalIcon />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem
                                destructive
                                onSelect={() =>
                                  setDeletingBusinessSystem(businessSystem)
                                }
                              >
                                {t("businessSystem.list.delete")}
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        )}
      </PageContent>

      <AlertDialog
        open={deletingBusinessSystem !== null}
        onOpenChange={(open) =>
          !open && !deleting && setDeletingBusinessSystem(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {deletingBusinessSystem
                ? t("businessSystem.delete.title", {
                    name: deletingBusinessSystem.name,
                  })
                : null}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("businessSystem.delete.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("businessSystem.delete.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void confirmDelete()}
            >
              {deleting
                ? t("businessSystem.delete.deleting")
                : t("businessSystem.delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
