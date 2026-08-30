/** Agent 单聊运行状态文案。 */
import type { TFunction } from "i18next"

import { AgentRunStatus } from "@/api"

/** 返回 Agent 单聊运行状态的用户文案。 */
export function agentRunStatusLabel(
  status: AgentRunStatus | null,
  t: TFunction<"inbox">,
) {
  switch (status) {
    case AgentRunStatus.AgentRunStatusQueued:
      return t("agentRunQueued")
    case AgentRunStatus.AgentRunStatusRunning:
      return t("agentRunRunning")
    case AgentRunStatus.AgentRunStatusFailed:
      return t("agentRunFailed")
    default:
      return ""
  }
}
