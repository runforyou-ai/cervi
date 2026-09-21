/** 通过模态框编辑 AI 员工表单中的 MCP 服务选择。 */
import { useTranslation } from "react-i18next"

import { listAgentMCPServerOptions } from "@/api"
import { AgentResourcePickerField } from "@/features/contacts/agents/agent-resource-picker-field"
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
  const { t } = useTranslation("contacts")
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
          detail: t("agents.mcp.tools", { count: service.toolCount }),
        }))
      }
      labels={{
        title: t("agents.mcp.title"),
        group: t("agents.mcp.services"),
        unconfigured: t("agents.mcp.unconfigured"),
        selected: (count, names) =>
          names === ""
            ? t("agents.mcp.selected", { count })
            : count === 1
              ? t("agents.mcp.selectedOne", { names })
              : t("agents.mcp.selectedNames", { names, count }),
        empty: t("agents.mcp.empty"),
        loadError: t("agents.mcp.loadError"),
      }}
    />
  )
}
