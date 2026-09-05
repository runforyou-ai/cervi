/** 移动端群聊历史、纯文本发送和解散后的只读状态。 */
import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import {
  ConversationStatus,
  ConversationType,
  type GroupConversationData,
} from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import {
  useOutgoingConversationMessages,
  type OutgoingConversationDraft,
} from "@/features/inbox/use-outgoing-conversation-messages"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 复用消息窗口和幂等重试，群聊解散后保留历史并关闭发送区。 */
export function MobileGroupThread({
  conversation,
  onUnavailable,
}: {
  conversation: GroupConversationData
  onUnavailable: () => void
}) {
  const { t } = useTranslation("mobile")
  const { identity } = useMobileWorkspace()
  const invalidate = useResourceInvalidator()
  const outgoing = useOutgoingConversationMessages()
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const archived =
    conversation.status === ConversationStatus.ConversationStatusArchived

  return (
    <>
      <ConversationTimeline
        conversationID={conversation.id}
        conversationType={ConversationType.ConversationTypeGroup}
        currentIdentityID={identity.user.identityId}
        requireWindowFocus={false}
        mentionNavigation={false}
        onUnavailable={onUnavailable}
        prepareSendRef={prepareSendRef}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={setRetryDraft}
        retryFailedMessageDisabled={
          archived || outgoing.messages.some((message) => message.status === "sending")
        }
      />
      {archived ? (
        <div
          className="shrink-0 border-t p-4 text-center text-sm text-muted-foreground"
          role="status"
        >
          {t("group.archived")}
        </div>
      ) : (
        <ConversationComposer
          conversationID={conversation.id}
          conversationType={ConversationType.ConversationTypeGroup}
          retryFailedMessage
          retryDraft={retryDraft}
          onRetryDraftHandled={() => setRetryDraft(null)}
          onBeforeSend={() => prepareSendRef.current?.() ?? Promise.resolve(true)}
          onSucceeded={() => void invalidate(resourceKeys.inbox())}
          onSending={outgoing.start}
          onSent={outgoing.succeed}
          onFailed={(clientMessageID) => {
            outgoing.fail(clientMessageID)
            // 发送被拒绝后立即同步群状态，及时关闭已解散群的发送区。
            void invalidate(resourceKeys.groupConversation(conversation.id))
          }}
        />
      )}
    </>
  )
}
