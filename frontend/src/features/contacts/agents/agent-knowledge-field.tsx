/** 通过模态框编辑 AI 员工绑定的企业本地知识库。 */
import { useTranslation } from "react-i18next"

import { listKnowledgeBases } from "@/api"
import { AgentResourcePickerField } from "@/features/contacts/agents/agent-resource-picker-field"
import { resourceKeys } from "@/hooks/resource-keys"

/** 显示已选知识库数量，在模态框中勾选 AI 员工可检索的知识库。 */
export function AgentKnowledgeField({
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
      resourceKey={resourceKeys.knowledgeBases()}
      load={() => listKnowledgeBases()}
      toOptions={(data) =>
        data.knowledgeBases.map((base) => ({
          id: base.id,
          name: base.name,
          detail: base.description,
        }))
      }
      labels={{
        title: t("agents.execution.knowledgeTitle"),
        group: t("agents.execution.knowledgeBases"),
        unconfigured: t("agents.execution.knowledgeUnconfigured"),
        selected: (count, names) =>
          names === ""
            ? t("agents.execution.knowledgeSelected", { count })
            : count === 1
              ? t("agents.execution.knowledgeSelectedOne", { names })
              : t("agents.execution.knowledgeSelectedNames", { names, count }),
        empty: t("agents.execution.knowledgeEmpty"),
        loadError: t("agents.execution.knowledgeLoadError"),
      }}
    />
  )
}
