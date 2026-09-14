/** 移动端真人与 AI 聊天共用的时间线、阅读进度、发送和失败重试。 */
import { useState } from "react"

import {
  ConversationType,
  type ConversationMessageData,
  type DirectTextMessageInput,
} from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import { useOutgoingMessages } from "@/features/inbox/outgoing-message-context"
import type { OutgoingConversationDraft } from "@/features/inbox/outgoing-message-store"
import { useConversationReadMarker } from "@/features/inbox/use-conversation-read-marker"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 草稿只展示本地发送状态，正式会话读取历史、推进已读并在前台轮询。 */
export function MobileIndividualThread({
  conversationID,
  conversationType = ConversationType.ConversationTypeDirect,
  peerIdentityID = "",
  sendIndividualMessage,
  enabled = Boolean(conversationID),
  disabledReason = null,
  lastReadMessageID = null,
}: {
  conversationID: string
  conversationType?: ConversationType
  peerIdentityID?: string
  enabled?: boolean
  disabledReason?: string | null
  lastReadMessageID?: string | null
  sendIndividualMessage?: (
    input: DirectTextMessageInput,
  ) => Promise<ConversationMessageData>
}) {
  const { identity } = useMobileWorkspace()
  const invalidate = useResourceInvalidator()
  // 真人草稿尚无会话编号，发送状态按对端身份分组。
  const outgoing = useOutgoingMessages(
    conversationID,
    peerIdentityID ? `draft:${peerIdentityID}` : "",
  )
  const markRead = useConversationReadMarker(conversationID, enabled)
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)

  return (
    <>
      <ConversationTimeline
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        requireWindowFocus={false}
        enabled={enabled}
        onReadMessage={enabled ? markRead : undefined}
        readThroughMessageID={lastReadMessageID}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={setRetryDraft}
        retryFailedMessageDisabled={
          Boolean(disabledReason) ||
          outgoing.messages.some((message) => message.status === "sending")
        }
      />
      <ConversationComposer
        conversationID={conversationID}
        conversationType={conversationType}
        retryFailedMessage
        disabledReason={disabledReason}
        retryDraft={retryDraft}
        onRetryDraftHandled={() => setRetryDraft(null)}
        sendIndividualMessage={sendIndividualMessage}
        onSucceeded={() => void invalidate(resourceKeys.inbox())}
        onSending={outgoing.start}
        onSent={outgoing.succeed}
        onFailed={outgoing.fail}
      />
    </>
  )
}
