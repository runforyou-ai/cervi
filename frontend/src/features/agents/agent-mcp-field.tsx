/** 通过模态框编辑 AI 员工表单中的 MCP 服务选择。 */
import { useTranslation } from "react-i18next"

import { listAgentMCPServerOptions } from "@/api"
import { AgentResourcePickerField } from "@/features/agents/agent-resource-picker-field"
import { resourceKeys } from "@/hooks/resource-keys"

/** 显示已选 MCP 服务数量，在模态框中勾选服务。 */
export function AgentMCPField({
  value,
  onChange,
  disabled,
}: {
  value: string[]
  onChange: (ids: string[]) => void
  disabled: boolean
}) {
  const { t } = useTranslation("agents")
  return (
    <AgentResourcePickerField
      value={value}
      onChange={onChange}
      disabled={disabled}
      resourceKey={resourceKeys.agentMCPServerOptions()}
      load={() => listAgentMCPServerOptions()}
      toOptions={(services) =>
        services.map((service) => ({
          id: service.id,
          name: service.name,
          detail: t("mcp.tools", { count: service.toolCount }),
        }))
      }
      labels={{
        title: t("mcp.title"),
        group: t("mcp.services"),
        unconfigured: t("mcp.unconfigured"),
        selected: (count, names) =>
          names === ""
            ? t("mcp.selected", { count })
            : count === 1
              ? t("mcp.selectedOne", { names })
              : t("mcp.selectedNames", { names, count }),
        empty: t("mcp.empty"),
        loadError: t("mcp.loadError"),
      }}
    />
  )
}
