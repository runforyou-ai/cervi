/** 桌面端独立窗口中的单个会话：会话头、消息时间线和默认收起的资料栏。 */
import { useEffect } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { Window } from "@wailsio/runtime"

import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { useAttachmentQueue } from "@/features/inbox/attachment-queue-context"
import { ConversationMain } from "@/features/inbox/conversation-main"
import { clearConversationResources } from "@/features/inbox/conversation-resources"
import { useOutgoingMessageStore } from "@/features/inbox/outgoing-message-context"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import { useConversationSummary } from "@/features/inbox/use-conversation-summary"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 读取会话摘要并在独立窗口内渲染会话主区，窗口标题跟随会话名称。 */
export function ConversationWindowPage({
  conversationId,
}: {
  conversationId: string
}) {
  const { t } = useTranslation(["inbox", "common"])
  const invalidate = useResourceInvalidator()
  const queryClient = useQueryClient()
  const { queue } = useAttachmentQueue()
  const outgoingStore = useOutgoingMessageStore()
  const conversationName = useConversationName()
  const summary = useConversationSummary(conversationId)
  const conversation = summary.data ?? null
  const title = conversation ? conversationName(conversation) : ""

  useEffect(() => {
    if (!title) return
    document.title = title
    void Window.SetTitle(title).catch((error: unknown) => {
      console.warn("更新会话窗口标题失败", { conversationId, error })
    })
  }, [conversationId, title])

  /** 消息或客服处理保存后刷新列表与详情。 */
  function refreshConversation(conversationID: string) {
    void invalidate(resourceKeys.inbox())
    void invalidate(resourceKeys.conversationSummary(conversationID))
  }

  /** 主动退群后清理该会话的本地资源并关闭窗口。 */
  function closeAfterGroupLeft(conversationID: string) {
    queue?.forgetConversation(conversationID)
    outgoingStore.forgetConversation(conversationID)
    clearConversationResources(queryClient, conversationID)
    void invalidate(resourceKeys.inbox())
    void Window.Close().catch((error: unknown) => {
      console.warn("关闭会话窗口失败", { conversationId: conversationID, error })
    })
  }

  if (!conversation) {
    return (
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
        {summary.loading ? (
          <LoadingIndicator>{t("messagesLoading")}</LoadingIndicator>
        ) : (
          <>
            <p>{t(summary.data === null ? "conversationUnavailable" : "conversationLoadError")}</p>
            {summary.data !== null ? (
              <Button variant="outline" size="sm" onClick={() => void summary.refresh()}>
                {t("common:actions.retry")}
              </Button>
            ) : null}
          </>
        )}
      </div>
    )
  }

  return (
    <section className="min-h-0 flex-1">
      <ConversationMain
        selection={{ kind: "conversation", conversation }}
        onSessionChanged={refreshConversation}
        onConversationChanged={refreshConversation}
        onGroupLeft={closeAfterGroupLeft}
        locateMessage={null}
      />
    </section>
  )
}
