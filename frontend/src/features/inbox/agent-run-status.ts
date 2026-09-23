/** Agent 单聊运行状态与助理在线状态文案。 */
import type { TFunction } from "i18next"

import { AgentRunStatus, AssistantPresence } from "@/api"

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

/** 返回助理不在线时的原因文案，在线或未知状态返回 null。 */
export function assistantPresenceLabel(
  presence: AssistantPresence | null | undefined,
  t: TFunction<"inbox">,
) {
  switch (presence) {
    case AssistantPresence.AssistantPresenceOffline:
      return t("assistantPresenceOffline")
    case AssistantPresence.AssistantPresencePaused:
      return t("assistantPresencePaused")
    case AssistantPresence.AssistantPresenceUnbound:
      return t("assistantPresenceUnbound")
    default:
      return null
  }
}
