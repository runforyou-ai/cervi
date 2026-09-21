/** MCP 服务列表页。 */
import { useEffect, useRef, useState } from "react"
import { PlugIcon, PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteMCPServer,
  isApiError,
  listMCPServers,
  refreshMCPServerTools,
  type MCPServerData,
} from "@/api"
import { ListActionButton } from "@/components/list-action-button"
import { ResourceListLayout } from "@/components/resource-list"
import { ResourceTable } from "@/components/resource-table"
import { PageHeader } from "@/components/page-header"
import { DeleteConfirmationDialog } from "@/components/delete-confirmation-dialog"
import { Button } from "@/components/ui/button"
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

  const deletion = useIntegrationDeletion<MCPServerData>({
    deleteItem: deleteMCPServer,
    listKey: resourceKeys.mcpServers(),
    detailKey: resourceKeys.mcpServer,
    relatedKeys: [resourceKeys.agentMCPServerOptions(), resourceKeys.agent()],
    entityName: "MCP 服务",
    successMessage: t("mcpServer.delete.success"),
    errorMessage: t("mcpServer.delete.error"),
  })

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={t("mcpServer.title")}
        description={t("mcpServer.description")}
      >
        <Button
          size="sm"
          variant="outline"
          disabled={submittingRefresh || !mcpServers.length || mcpServers.some((server) => server.toolsUpdating)}
          onClick={() => void updateTools()}
        >
          {t("mcpServer.tools.refresh")}
        </Button>
        <Button variant="ghost" size="icon-sm" asChild>
          <Link
            to="/settings/mcp-servers/new"
            aria-label={t("mcpServer.list.create")}
            title={t("mcpServer.list.create")}
          >
            <PlusIcon />
          </Link>
        </Button>
      </PageHeader>
      <ResourceListLayout
        loading={showLoading}
        error={Boolean(loadError) && !data}
        errorMessage={t("mcpServer.list.loadError")}
        onRetry={() => void refresh()}
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "server",
              header: t("mcpServer.list.columns.name"),
              cellClassName: "min-w-0",
              cell: (mcpServer) => (
                <div className="flex min-w-0 items-center gap-3">
                  <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
                    <PlugIcon className="size-4.5" aria-hidden="true" />
                  </span>
                  <span className="grid min-w-0 gap-0.5 leading-tight">
                    <span className="truncate font-medium">{mcpServer.name}</span>
                    <span className="flex min-w-0 items-center text-xs text-muted-foreground">
                      <MCPServerToolsCell server={mcpServer} />
                    </span>
                  </span>
                </div>
              ),
            },
          ]}
          rows={mcpServers}
          rowKey={(mcpServer) => mcpServer.id}
          empty={t("mcpServer.list.empty")}
          onRowActivate={(mcpServer) =>
            navigate(`/settings/mcp-servers/${mcpServer.id}`)
          }
          actions={(mcpServer) => ({
            // 操作靠右对齐，与企业成员列表一致。
            primary: (
              <div className="ml-auto flex items-center gap-1">
                <MCPServerTestButton serverId={mcpServer.id} />
                <ListActionButton
                  tone="destructive"
                  onClick={() => deletion.select(mcpServer)}
                >
                  {t("common:actions.delete")}
                </ListActionButton>
              </div>
            ),
          })}
        />
      </ResourceListLayout>

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
