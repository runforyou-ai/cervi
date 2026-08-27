/** 连接器列表页。 */
import { useEffect, useRef, useState } from "react"
import { LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  IntegrationConnectionStatus,
  deleteIntegrationConnection,
  isApiError,
  listIntegrationConnections,
  type IntegrationConnectionStatusId,
  type IntegrationConnectionSummaryData,
} from "@/api"
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
import { connectorTypeConfigs } from "@/features/integrations/connectors/connector-options"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 选择连接状态的展示样式。 */
function statusVariant(status: IntegrationConnectionStatusId) {
  if (status === IntegrationConnectionStatus.IntegrationConnectionStatusAvailable) {
    return "success" as const
  }
  if (
    status === IntegrationConnectionStatus.IntegrationConnectionStatusUnavailable
  ) {
    return "destructive" as const
  }
  return "muted" as const
}

/** 显示并管理当前企业的连接器。 */
export function ConnectorListPage() {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [deletingConnection, setDeletingConnection] =
    useState<IntegrationConnectionSummaryData | null>(null)
  const [deleting, setDeleting] = useState(false)
  const mounted = useRef(true)
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.connectors(),
    () => listIntegrationConnections(),
  )
  const invalidate = useResourceInvalidator()
  const showLoading = loading || (Boolean(error) && refreshing)
  const connections = data?.connections ?? []

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 删除选中的连接器。 */
  async function confirmDelete() {
    if (!deletingConnection || deleting) return
    setDeleting(true)
    try {
      await deleteIntegrationConnection(deletingConnection.id)
      if (!mounted.current) return
      void refresh()
      void invalidate(resourceKeys.connector(deletingConnection.id))
      setDeletingConnection(null)
      toast.success(t("connectors.delete.success"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("连接器删除失败", {
        connection_id: deletingConnection.id,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError)
          : t("connectors.delete.error"),
      )
    } finally {
      if (mounted.current) setDeleting(false)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("connectors.title")}>
        <Button size="sm" asChild>
          <Link to="/integrations/connectors/new">
            {t("connectors.list.create")}
          </Link>
        </Button>
      </PageHeader>
      <PageContent>
        {showLoading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("connectors.loading")}
          </div>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("connectors.list.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refresh()}
            >
              {t("connectors.retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("connectors.list.columns.name")}</TableHead>
                  <TableHead>{t("connectors.list.columns.type")}</TableHead>
                  <TableHead>{t("connectors.list.columns.description")}</TableHead>
                  <TableHead>{t("connectors.list.columns.status")}</TableHead>
                  <TableHead>{t("connectors.list.columns.lastTestedAt")}</TableHead>
                  <TableHead className="text-right">
                    {t("connectors.list.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {connections.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={6}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {t("connectors.list.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  connections.map((connection) => (
                    <TableRow key={connection.id}>
                      <TableCell className="font-medium">
                        <SelectableText>{connection.name}</SelectableText>
                      </TableCell>
                      <TableCell>
                        {t(connectorTypeConfigs[connection.type].nameKey)}
                      </TableCell>
                      <TableCell className="max-w-sm text-muted-foreground">
                        <SelectableText className="block max-w-sm truncate">
                          {connection.description || "—"}
                        </SelectableText>
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          variant={statusVariant(connection.status)}
                          showDot={false}
                        >
                          {t(`connectors.status.${connection.status}`)}
                        </StatusBadge>
                      </TableCell>
                      <TableCell className="text-muted-foreground whitespace-nowrap">
                        {connection.lastTestedAt
                          ? formatDateTime(connection.lastTestedAt)
                          : "—"}
                      </TableCell>
                      <TableCell className="text-right whitespace-nowrap">
                        <div className="flex justify-end gap-2">
                          <Button variant="outline" size="sm" asChild>
                            <Link
                              to={`/integrations/connectors/${connection.id}`}
                            >
                              {t("connectors.list.edit")}
                            </Link>
                          </Button>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                aria-label={t("connectors.list.more")}
                                title={t("connectors.list.more")}
                              >
                                <MoreHorizontalIcon />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem
                                className="text-destructive focus:text-destructive"
                                onSelect={() => setDeletingConnection(connection)}
                              >
                                {t("connectors.list.delete")}
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
        open={deletingConnection !== null}
        onOpenChange={(open) => {
          if (!open && !deleting) setDeletingConnection(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("connectors.delete.title", {
                name: deletingConnection?.name ?? "",
              })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("connectors.delete.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("connectors.delete.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={(event) => {
                event.preventDefault()
                void confirmDelete()
              }}
            >
              {deleting ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : null}
              {deleting
                ? t("connectors.delete.deleting")
                : t("connectors.delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
