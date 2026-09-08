/** 清理失权会话的共享读取资源，阻止在途查询重新安装旧结果。 */
import type { QueryClient } from "@tanstack/react-query"
import { resourceKeys } from "@/hooks/resource-keys"

/** 保留独立摘要的不可用结果，移除正文及衍生缓存并重读列表。 */
export function clearConversationResources(client: QueryClient, conversationID: string) {
  const keys = [
    resourceKeys.conversationMessages(conversationID),
    resourceKeys.conversationMessagePage(conversationID),
    resourceKeys.conversationMessageContext(conversationID),
    resourceKeys.conversationMessageReferences(conversationID),
    resourceKeys.conversationNavigation(conversationID),
    resourceKeys.conversationMentions(conversationID),
    resourceKeys.groupConversation(conversationID),
    resourceKeys.customerDeliveries(conversationID),
    resourceKeys.attachmentStates(conversationID),
  ]
  for (const queryKey of keys) client.removeQueries({ queryKey })
  client.removeQueries({ queryKey: resourceKeys.attachmentDownload(conversationID) })
  client.removeQueries({ queryKey: resourceKeys.directConversation() })
  client.removeQueries({ queryKey: resourceKeys.inboxConversations() })
  void client.resetQueries({ queryKey: resourceKeys.inbox() })
}
