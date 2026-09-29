/** 本人对群聊、单聊与 AI 聊天的归档切换。 */
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, updateConversationArchive } from "@/api"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 归档状态变化后需要重读的会话摘要、群资料、聊天列表、提醒数与已归档聊天。 */
export function conversationArchiveKeys(conversationId: string) {
  return [
    resourceKeys.conversationSummary(conversationId),
    resourceKeys.groupConversation(conversationId),
    resourceKeys.inbox(),
    resourceKeys.inboxAttention(),
    resourceKeys.archivedConversations(),
  ]
}

/** 保存会话的归档状态，保存后重读相关资源，当前页面保持不变。 */
export function useConversationArchive() {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [saving, setSaving] = useState(false)

  /** 保存指定会话的归档状态，失败时提示原因。 */
  async function save(conversationId: string, archived: boolean) {
    setSaving(true)
    try {
      await updateConversationArchive(conversationId, { archived })
      await Promise.all(conversationArchiveKeys(conversationId).map((key) => invalidate(key)))
    } catch (error) {
      console.warn("更新会话归档失败", { conversationId, error })
      if (!recoverSession(error, navigate)) {
        toast.error(isApiError(error) ? apiErrorMessage(error) : t("conversationArchiveError"))
      }
    } finally {
      setSaving(false)
    }
  }

  return { saving, save }
}
