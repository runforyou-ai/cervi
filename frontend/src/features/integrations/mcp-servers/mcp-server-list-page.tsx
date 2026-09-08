/** MCP 服务列表页。 */
import { useEffect, useRef, useState } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteMCPServer,
  isApiError,
  listMCPServers,
  type MCPServer,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
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
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 显示当前企业配置的 MCP 服务。 */
export function MCPServerListPage() {
  const { t } = useTranslation(["integrations", "common"])
  const navigate = useNavigate()
  const [deletingMCPServer, setDeletingMCPServer] =
    useState<MCPServer | null>(null)
  const [deleting, setDeleting] = useState(false)
  const mounted = useRef(true)
  const {
    data,
    loading,
    refreshing,
    error: loadError,
    refresh,
  } = useResource(resourceKeys.mcpServers(), () => listMCPServers())
  const invalidate = useResourceInvalidator()
  const showLoading = loading || (Boolean(loadError) && refreshing)
  const mcpServers = data?.mcpServers ?? []

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 删除选中的 MCP 服务。 */
  async function confirmDelete() {
    if (!deletingMCPServer || deleting) return
    setDeleting(true)
    try {
      await deleteMCPServer(deletingMCPServer.id)
      if (!mounted.current) return
      void refresh()
      void invalidate(resourceKeys.mcpServer(deletingMCPServer.id))
      console.info("MCP 服务已删除", {
        mcp_server_id: deletingMCPServer.id,
      })
      setDeletingMCPServer(null)
      toast.success(t("mcpServer.delete.success"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("MCP 服务删除失败", {
        mcp_server_id: deletingMCPServer.id,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError)
          : t("mcpServer.delete.error"),
      )
    } finally {
      if (mounted.current) setDeleting(false)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("mcpServer.title")}>
        <Button size="sm" asChild>
          <Link to="/integrations/mcp-servers/new">
            {t("mcpServer.list.create")}
          </Link>
        </Button>
      </PageHeader>
      <PageContent>
        {showLoading ? (
          <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
            {t("common:status.loading")}
          </LoadingIndicator>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("mcpServer.list.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refresh()}
            >
              {t("common:actions.retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("mcpServer.list.columns.name")}</TableHead>
                  <TableHead>{t("mcpServer.list.columns.url")}</TableHead>
                  <TableHead>{t("mcpServer.list.columns.serverType")}</TableHead>
                  <TableHead className="w-px">
                    {t("common:table.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {mcpServers.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={4}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {t("mcpServer.list.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  mcpServers.map((mcpServer) => (
                    <TableRow key={mcpServer.id}>
                      <TableCell className="font-medium">
                        <SelectableText>{mcpServer.name}</SelectableText>
                      </TableCell>
                      <TableCell className="max-w-xl text-muted-foreground">
                        <SelectableText>{mcpServer.url}</SelectableText>
                      </TableCell>
                      <TableCell>
                        <SelectableText>{mcpServer.serverType}</SelectableText>
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        <div className="inline-flex gap-2">
                          <Button variant="outline" size="sm" asChild>
                            <Link
                              to={`/integrations/mcp-servers/${mcpServer.id}`}
                            >
                              {t("common:actions.edit")}
                            </Link>
                          </Button>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                aria-label={t("common:actions.more")}
                                title={t("common:actions.more")}
                              >
                                <MoreHorizontalIcon />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem
                                destructive
                                onSelect={() =>
                                  setDeletingMCPServer(mcpServer)
                                }
                              >
                                {t("common:actions.delete")}
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
        open={deletingMCPServer !== null}
        onOpenChange={(open) =>
          !open && !deleting && setDeletingMCPServer(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {deletingMCPServer
                ? t("mcpServer.delete.title", {
                    name: deletingMCPServer.name,
                  })
                : null}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("mcpServer.delete.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("common:actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void confirmDelete()}
            >
              {deleting
                ? t("common:actions.deleting")
                : t("common:actions.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
