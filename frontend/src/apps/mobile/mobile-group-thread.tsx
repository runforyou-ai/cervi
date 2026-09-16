/** 移动端群聊历史、文本与附件发送、引用与提及输入、阅读进度和解散后的只读状态。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import {
  ConversationStatus,
  ConversationType,
  type ConversationMessageReference,
  type GroupConversationData,
} from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import {
  ConversationTimeline,
  type ConversationLocateTarget,
} from "@/features/inbox/conversation-timeline"
import { useOutgoingMessages } from "@/features/inbox/outgoing-message-context"
import type { OutgoingConversationDraft } from "@/features/inbox/outgoing-message-store"
import { useConversationReadMarker } from "@/features/inbox/use-conversation-read-marker"
import { useRecentConversations } from "@/features/inbox/use-recent-conversations"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 复用消息窗口、已读与提及导航，按需定位原消息，群聊解散后保留历史并关闭发送区。 */
export function MobileGroupThread({
  conversation,
  active = true,
  locateMessage = null,
  onUnavailable,
}: {
  conversation: GroupConversationData
  active?: boolean
  locateMessage?: ConversationLocateTarget | null
  onUnavailable: () => void
}) {
  const { t } = useTranslation("mobile")
  const { identity } = useMobileWorkspace()
  const invalidate = useResourceInvalidator()
  const outgoing = useOutgoingMessages(conversation.id)
  const markRead = useConversationReadMarker(conversation.id, active)
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const [replyTo, setReplyTo] =
    useState<ConversationMessageReference | null>(null)
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const archived =
    conversation.status === ConversationStatus.ConversationStatusArchived
  const { record: recordRecentConversation } = useRecentConversations(
    identity.user.identityId,
  )

  useEffect(() => {
    // 群聊打开后记入本机最近打开。
    recordRecentConversation(conversation.id)
  }, [conversation.id, recordRecentConversation])

  return (
    <>
      <ConversationTimeline
        conversationID={conversation.id}
        enabled={active}
        conversationType={ConversationType.ConversationTypeGroup}
        currentUser={identity.user}
        requireWindowFocus={false}
        onUnavailable={onUnavailable}
        onReadMessage={markRead}
        onReplyMessage={archived ? undefined : setReplyTo}
        groupParticipants={conversation.participants}
        prepareSendRef={prepareSendRef}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={setRetryDraft}
        retryFailedMessageDisabled={archived}
        locateMessage={locateMessage}
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
          replyTo={replyTo}
          groupParticipants={conversation.participants}
          currentIdentityID={identity.user.identityId}
          onRetryDraftHandled={() => setRetryDraft(null)}
          onReplyToChange={setReplyTo}
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
