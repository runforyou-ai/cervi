/** MCP 工具数量与目录提示。 */
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import type { MCPServerData } from "@/api"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

/** 在固定宽度内显示数量、更新状态及工具目录。 */
export function MCPServerToolsCell({ server }: { server: MCPServerData }) {
  const { t, i18n } = useTranslation("integrations")
  const tools = server.tools
  const label = server.toolsUpdating
    ? t("mcpServer.tools.updating")
    : server.toolsError || (server.toolsUpdatedAt
      ? t("mcpServer.tools.count", { count: tools.length })
      : t("mcpServer.tools.pending"))

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={label}
          className={`inline-flex h-8 min-w-8 items-center justify-start rounded-sm tabular-nums outline-none focus-visible:ring-2 focus-visible:ring-ring ${server.toolsError ? "text-destructive" : "text-muted-foreground"}`}
        >
          {server.toolsUpdating ? (
            <LoaderCircleIcon className="size-4 animate-spin" aria-hidden="true" />
          ) : server.toolsUpdatedAt ? tools.length.toLocaleString(i18n.language) : "—"}
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom" align="start" sideOffset={4} className="max-w-sm text-left text-wrap">
        <div className="max-h-72 space-y-3 overflow-y-auto overscroll-contain">
          {server.toolsUpdating ? <p>{t("mcpServer.tools.updating")}</p> : null}
          {server.toolsError ? <p>{t("mcpServer.tools.failed", { message: server.toolsError })}</p> : null}
          {tools.length ? (
            <ul className="space-y-3">
              {tools.map((tool) => (
                <li key={tool.name} className="space-y-1 break-words">
                  <p className="font-mono font-medium">{tool.name}</p>
                  <p className="whitespace-pre-wrap opacity-80">{tool.description || t("mcpServer.tools.noDescription")}</p>
                </li>
              ))}
            </ul>
          ) : (
            <p>{server.toolsUpdatedAt ? t("mcpServer.tools.empty") : t("mcpServer.tools.pending")}</p>
          )}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
