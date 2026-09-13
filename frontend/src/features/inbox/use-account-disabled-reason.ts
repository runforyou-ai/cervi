/** 按会话对端账号状态生成发送框禁用提示。 */
import { useTranslation } from "react-i18next"

import { UserStatus } from "@/api"

/** 对端 AI 员工或成员已禁用时返回发送框提示，其余返回 null。 */
export function useAccountDisabledReason(
  conversation: {
    agent: { agentStatus: UserStatus } | null
    direct: { peerStatus: UserStatus } | null
  } | null,
) {
  const { t } = useTranslation("inbox")
  if (conversation?.agent?.agentStatus === UserStatus.UserStatusInactive) {
    return t("agentDisabledUnavailable")
  }
  if (conversation?.direct?.peerStatus === UserStatus.UserStatusInactive) {
    return t("directPeerDisabledUnavailable")
  }
  return null
}
