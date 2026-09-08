/** 连接器类型的界面配置。 */
import {
  IntegrationConnectionType,
  type IntegrationConnectionTypeId,
} from "@/api"

type ConnectorTypeConfig = {
  nameKey: `connectors.types.${"dify" | "n8n" | "ragflow"}`
  apiURLLabelKey: `connectors.form.${"apiUrl" | "instanceUrl"}`
  defaultAPIURL: string
}

/** 连接器类型的显示顺序。 */
export const connectorTypeOrder: IntegrationConnectionTypeId[] = [
  IntegrationConnectionType.IntegrationConnectionTypeDify,
  IntegrationConnectionType.IntegrationConnectionTypeN8N,
  IntegrationConnectionType.IntegrationConnectionTypeRAGFlow,
]

/** 各连接器类型的表单显示配置。 */
export const connectorTypeConfigs: Record<
  IntegrationConnectionTypeId,
  ConnectorTypeConfig
> = {
  [IntegrationConnectionType.IntegrationConnectionTypeDify]: {
    nameKey: "connectors.types.dify",
    apiURLLabelKey: "connectors.form.apiUrl",
    defaultAPIURL: "https://api.dify.ai/v1",
  },
  [IntegrationConnectionType.IntegrationConnectionTypeN8N]: {
    nameKey: "connectors.types.n8n",
    apiURLLabelKey: "connectors.form.instanceUrl",
    defaultAPIURL: "",
  },
  [IntegrationConnectionType.IntegrationConnectionTypeRAGFlow]: {
    nameKey: "connectors.types.ragflow",
    apiURLLabelKey: "connectors.form.instanceUrl",
    defaultAPIURL: "",
  },
}
