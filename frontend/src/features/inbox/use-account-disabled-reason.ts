/** 按会话对端账号状态生成发送框禁用提示。 */
import { useTranslation } from "react-i18next"

import { AssistantPresence, OrganizationIdentityType, UserStatus } from "@/api"

/** 对端 AI 员工、助理或成员已禁用，或助理已暂停、绑定电脑已撤销时返回发送框提示，其余返回 null。 */
export function useAccountDisabledReason(
  conversation: {
    agent: {
      agentStatus: UserStatus
      agentType: OrganizationIdentityType
      assistantPresence: AssistantPresence | null
    } | null
    direct: { peerStatus: UserStatus } | null
  } | null,
) {
  const { t } = useTranslation("inbox")
  if (conversation?.agent?.agentStatus === UserStatus.UserStatusInactive) {
    return t(
      conversation.agent.agentType === OrganizationIdentityType.OrganizationIdentityTypeAssistant
        ? "assistantDisabledUnavailable"
        : "agentDisabledUnavailable",
    )
  }
  if (conversation?.agent?.assistantPresence === AssistantPresence.AssistantPresencePaused) {
    return t("assistantPausedUnavailable")
  }
  if (conversation?.agent?.assistantPresence === AssistantPresence.AssistantPresenceUnbound) {
    return t("assistantUnboundUnavailable")
  }
  if (conversation?.direct?.peerStatus === UserStatus.UserStatusInactive) {
    return t("directPeerDisabledUnavailable")
  }
  return null
}
