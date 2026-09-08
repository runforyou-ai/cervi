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
  refreshMCPServerTools,
  type MCPServer,
} from "@/api"
import { ResourceContent } from "@/components/resource-content"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { SelectableText } from "@/components/selectable-text"
import { DeleteConfirmationDialog } from "@/components/delete-confirmation-dialog"
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
import { MCPServerToolsCell } from "@/features/integrations/mcp-servers/mcp-server-tools-cell"
import { MCPServerTestButton } from "@/features/integrations/mcp-servers/mcp-server-test-button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { useIntegrationDeletion } from "@/features/integrations/use-integration-deletion"

/** 显示当前企业配置的 MCP 服务。 */
export function MCPServerListPage() {
  const { t } = useTranslation(["integrations", "common"])
  const navigate = useNavigate()
  const [submittingRefresh, setSubmittingRefresh] = useState(false)
  const mounted = useRef(true)
  const {
    data,
    loading,
    refreshing,
    error: loadError,
    refresh,
  } = useResource(resourceKeys.mcpServers(), () => listMCPServers(), {
    staleTime: 0,
    refetchInterval: (data) => data?.mcpServers.some((server) => server.toolsUpdating) ? 1000 : false,
    refetchOnWindowFocus: true,
  })
  const showLoading = loading || (Boolean(loadError) && !data && refreshing)
  const mcpServers = data?.mcpServers ?? []

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 提交全部服务的更新任务，并读取服务端返回的更新状态。 */
  async function updateTools() {
    if (submittingRefresh) return
    setSubmittingRefresh(true)
    try {
      await refreshMCPServerTools()
      await refresh()
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("mcpServer.tools.submitError"))
    } finally {
      if (mounted.current) setSubmittingRefresh(false)
    }
  }

  const deletion = useIntegrationDeletion<MCPServer>({
    deleteItem: deleteMCPServer,
    listKey: resourceKeys.mcpServers(),
    detailKey: resourceKeys.mcpServer,
    entityName: "MCP 服务",
    successMessage: t("mcpServer.delete.success"),
    errorMessage: t("mcpServer.delete.error"),
  })

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("mcpServer.title")}>
        <Button
          size="sm"
          variant="outline"
          disabled={submittingRefresh || !mcpServers.length || mcpServers.some((server) => server.toolsUpdating)}
          onClick={() => void updateTools()}
        >
          {t("mcpServer.tools.refresh")}
        </Button>
        <Button size="sm" asChild>
          <Link to="/integrations/mcp-servers/new">{t("mcpServer.list.create")}</Link>
        </Button>
      </PageHeader>
      <PageContent>
        <ResourceContent
          loading={showLoading}
          error={Boolean(loadError) && !data}
          errorMessage={t("mcpServer.list.loadError")}
          onRetry={() => void refresh()}
        >
          <div className="overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("mcpServer.list.columns.name")}</TableHead>
                  <TableHead>{t("mcpServer.list.columns.serverType")}</TableHead>
                  <TableHead>{t("mcpServer.list.columns.url")}</TableHead>
                  <TableHead className="w-20">{t("mcpServer.list.columns.tools")}</TableHead>
                  <TableHead className="w-px">
                    {t("common:table.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {mcpServers.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={5}
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
                      <TableCell>
                        <SelectableText>{mcpServer.serverType}</SelectableText>
                      </TableCell>
                      <TableCell className="max-w-xl text-muted-foreground">
                        <SelectableText>{mcpServer.url}</SelectableText>
                      </TableCell>
                      <TableCell><MCPServerToolsCell server={mcpServer} /></TableCell>
                      <TableCell className="whitespace-nowrap">
                        <div className="inline-flex gap-2">
                          <MCPServerTestButton serverId={mcpServer.id} />
                          <Button variant="outline" size="sm" asChild>
                            <Link to={`/integrations/mcp-servers/${mcpServer.id}`}>
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
                                onSelect={() => deletion.select(mcpServer)}
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
        </ResourceContent>
      </PageContent>

      <DeleteConfirmationDialog
        open={deletion.item !== null}
        pending={deletion.pending}
        title={
          deletion.item ? t("mcpServer.delete.title", { name: deletion.item.name }) : ""
        }
        description={t("mcpServer.delete.description")}
        onOpenChange={(open) => {
          if (!open) deletion.select(null)
        }}
        onConfirm={() => void deletion.confirm()}
      />
    </div>
  )
}
