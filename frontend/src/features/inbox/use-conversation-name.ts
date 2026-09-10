/** 会话显示名解析。 */
import { useCallback } from "react"
import { useTranslation } from "react-i18next"

import {
  isAgentInboxConversation,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type InboxConversation,
} from "@/api"

/** 会话在列表和主区中的显示名。 */
export function useConversationName() {
  const { t } = useTranslation("inbox")
  return useCallback(
    (conversation: InboxConversation) => {
      if (isAgentInboxConversation(conversation))
        return conversation.agent.title
      if (isDirectInboxConversation(conversation)) {
        return conversation.direct.peerName.trim() || t("unknownSender")
      }
      if (isCustomerInboxConversation(conversation)) {
        return (
          conversation.customer.contactName?.trim() || t("anonymousVisitor")
        )
      }
      if (isGroupInboxConversation(conversation)) {
        return conversation.group.title.trim() || t("unknownSender")
      }
      return t("unknownSender")
    },
    [t],
  )
}
