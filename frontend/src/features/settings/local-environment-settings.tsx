/** 本机设置：这台电脑为助理提供的运行环境与本地 MCP 服务，当前页签与地址同步。 */
import { useEffect, useState } from "react"
import { BlocksIcon, PackageIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearchParams } from "react-router"
import { toast } from "sonner"

import {
  getLocalEnvironment,
  isApiError,
  LocalToolchainFailure,
  LocalToolchainState,
  openLocalToolchainFolder,
  removeLocalMCPServer,
  updateLocalToolchain,
  type LocalEnvironmentData,
  type LocalMCPServerData,
} from "@/api"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { ResourceContent } from "@/components/resource-content"
import { ResourceListFrame } from "@/components/resource-list"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { StatusBadge } from "@/components/status-badge"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 本机设置的页签，首项为缺省页签。 */
const localTabs = ["toolchain", "mcp"] as const

/** 按地址中的页签显示运行环境或本地 MCP 服务。 */
export function LocalEnvironmentSettings() {
  const { t } = useTranslation("settings")
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = localTabs.find((value) => value === searchParams.get("tab")) ?? localTabs[0]
  const environment = useResource(resourceKeys.localEnvironment(), () => getLocalEnvironment(), {
    staleTime: 0,
  })

  // 缺省或无效页签统一写回地址，刷新时恢复同一页签。
  useEffect(() => {
    if (searchParams.get("tab") === tab) return
    const next = new URLSearchParams(searchParams)
    next.set("tab", tab)
    setSearchParams(next, { replace: true })
  }, [searchParams, setSearchParams, tab])

  return (
    <Tabs
      value={tab}
      onValueChange={(value) => {
        const next = new URLSearchParams(searchParams)
        next.set("tab", value)
        setSearchParams(next, { replace: true })
      }}
    >
      <TabsList>
        <TabsTrigger value="toolchain">{t("local.tabs.toolchain")}</TabsTrigger>
        <TabsTrigger value="mcp">{t("local.tabs.mcp")}</TabsTrigger>
      </TabsList>
      <ResourceContent resources={environment} errorMessage={t("local.loadError")}>
        {environment.data ? (
          <>
            <TabsContent value="toolchain" forceMount className="mt-6 data-[state=inactive]:hidden">
              <ToolchainSettings environment={environment.data} />
            </TabsContent>
            <TabsContent value="mcp" forceMount className="mt-6 data-[state=inactive]:hidden">
              <LocalMCPServers servers={environment.data.mcpServers} />
            </TabsContent>
          </>
        ) : null}
      </ResourceContent>
    </Tabs>
  )
}

/** 展示运行环境的状态、各组件版本与安装位置，并提供打开位置与检查更新。 */
function ToolchainSettings({ environment }: { environment: LocalEnvironmentData }) {
  const { t } = useTranslation(["settings", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [updating, setUpdating] = useState(false)
  const { toolchain } = environment
  const busy = updating || toolchain.updating
  const components = [
    { name: "uv", version: environment.uvVersion },
    { name: "Node.js", version: environment.nodeVersion },
    { name: "Python", version: environment.pythonVersion },
  ]

  /** 检查并安装最新版本，完成后刷新本机环境。 */
  async function update() {
    setUpdating(true)
    try {
      const result = await updateLocalToolchain()
      toast.success(result.updated ? t("local.toolchain.updated") : t("local.toolchain.upToDate"))
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("更新运行环境失败", error)
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("local.toolchain.updateError"))
    } finally {
      setUpdating(false)
      void invalidate(resourceKeys.localEnvironment())
    }
  }

  /** 在系统文件管理器中打开安装位置。 */
  async function openFolder() {
    try {
      await openLocalToolchainFolder()
    } catch (error) {
      console.warn("打开运行环境安装位置失败", error)
      toast.error(t("local.toolchain.openError"))
    }
  }

  return (
    <FieldGroup className="gap-8">
      <Field>
        <FieldLabel>{t("local.toolchain.status")}</FieldLabel>
        <div>
          <ToolchainStatus environment={environment} updating={busy} />
        </div>
      </Field>
      <Field>
        <FieldLabel>{t("local.toolchain.components")}</FieldLabel>
        <ResourceListFrame>
          <ResourceTable
            hideHeader
            columns={[
              {
                key: "component",
                header: t("local.toolchain.components"),
                cellClassName: "min-w-0",
                cell: (component) => (
                  <ResourceRowIdentity
                    icon={PackageIcon}
                    name={component.name}
                    secondary={component.version || t("local.toolchain.notInstalled")}
                  />
                ),
              },
            ]}
            rows={components}
            rowKey={(component) => component.name}
            empty={null}
          />
        </ResourceListFrame>
      </Field>
      <Field>
        <FieldLabel>{t("local.toolchain.location")}</FieldLabel>
        <div className="flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2">
          <code className="flex min-h-8 min-w-0 flex-1 items-center font-mono text-sm break-all">
            {environment.location}
          </code>
          <Button type="button" variant="outline" size="sm" className="shrink-0" onClick={() => void openFolder()}>
            {t("local.toolchain.open")}
          </Button>
        </div>
      </Field>
      <div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={busy || toolchain.state !== LocalToolchainState.LocalToolchainStateReady}
          onClick={() => void update()}
        >
          {busy ? t("local.toolchain.updating") : t("local.toolchain.update")}
        </Button>
      </div>
    </FieldGroup>
  )
}

/** 展示运行环境的准备、更新或失败状态。 */
function ToolchainStatus({ environment, updating }: { environment: LocalEnvironmentData; updating: boolean }) {
  const { t } = useTranslation("settings")
  const { toolchain } = environment
  if (updating) {
    return <StatusBadge variant="muted">{t("local.toolchain.states.updating")}</StatusBadge>
  }
  switch (toolchain.state) {
    case LocalToolchainState.LocalToolchainStateReady:
      return <StatusBadge variant="success">{t("local.toolchain.states.ready")}</StatusBadge>
    case LocalToolchainState.LocalToolchainStateFailed:
      return (
        <StatusBadge variant="destructive">
          {toolchain.failure === LocalToolchainFailure.LocalToolchainFailureDownload
            ? t("local.toolchain.states.downloadFailed")
            : toolchain.failure === LocalToolchainFailure.LocalToolchainFailureVerify
              ? t("local.toolchain.states.verifyFailed")
              : t("local.toolchain.states.installFailed")}
        </StatusBadge>
      )
    default:
      return <StatusBadge variant="muted">{t("local.toolchain.states.preparing")}</StatusBadge>
  }
}

/** 列出本地 MCP 服务，可删除其中一个。 */
function LocalMCPServers({ servers }: { servers: LocalMCPServerData[] }) {
  const { t } = useTranslation("settings")
  const removal = useConfirmedAction<LocalMCPServerData>({
    action: (server) => removeLocalMCPServer(server.name),
    invalidateKeys: () => [resourceKeys.localEnvironment()],
    successMessage: () => t("local.mcp.remove.success"),
    errorMessage: () => t("local.mcp.remove.error"),
    logLabel: "删除本地 MCP 服务",
  })

  return (
    <>
      <ResourceListFrame>
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "server",
              header: t("local.mcp.columns.name"),
              cellClassName: "min-w-0",
              cell: (server) => (
                <ResourceRowIdentity
                  icon={BlocksIcon}
                  name={server.name}
                  description={[server.command, ...server.args].join(" ")}
                />
              ),
            },
          ]}
          rows={servers}
          rowKey={(server) => server.name}
          empty={t("local.mcp.empty")}
          rowActions={(server) => [
            {
              key: "remove",
              label: t("local.mcp.remove.action"),
              destructive: true,
              separatorBefore: true,
              onSelect: () => removal.select(server),
            },
          ]}
        />
      </ResourceListFrame>
      <ConfirmationDialog
        {...removal.dialog}
        title={t("local.mcp.remove.title", { name: removal.item?.name ?? "" })}
        description={t("local.mcp.remove.description")}
        pendingLabel={t("local.mcp.remove.pending")}
      />
    </>
  )
}
