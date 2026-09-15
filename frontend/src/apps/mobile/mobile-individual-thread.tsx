/** 移动端真人与 AI 聊天共用的时间线、阅读进度、文本与附件发送和失败重试。 */
import { useRef, useState } from "react"

import {
  ConversationType,
  type ConversationMessageData,
  type ConversationMessageReference,
  type DirectTextMessageInput,
  type InboxConversation,
} from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { useAttachmentQueue } from "@/features/inbox/attachment-queue-context"
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
  attachmentAgentIdentityID,
  onAttachmentConversationCreated,
  enabled = Boolean(conversationID),
  disabledReason = null,
  lastReadMessageID = null,
}: {
  conversationID: string
  conversationType?: ConversationType
  peerIdentityID?: string
  attachmentAgentIdentityID?: string
  onAttachmentConversationCreated?: (conversation: InboxConversation) => void
  enabled?: boolean
  disabledReason?: string | null
  lastReadMessageID?: string | null
  sendIndividualMessage?: (
    input: DirectTextMessageInput,
  ) => Promise<ConversationMessageData>
}) {
  const { identity } = useMobileWorkspace()
  const { queue, jobs } = useAttachmentQueue()
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const invalidate = useResourceInvalidator()
  // 真人草稿尚无会话编号，发送状态按对端身份分组。
  const outgoing = useOutgoingMessages(
    conversationID,
    peerIdentityID ? `draft:${peerIdentityID}` : "",
  )
  const markRead = useConversationReadMarker(conversationID, enabled)
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const [replyTo, setReplyTo] =
    useState<ConversationMessageReference | null>(null)

  return (
    <>
      <ConversationTimeline
        prepareSendRef={prepareSendRef}
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        requireWindowFocus={false}
        enabled={enabled}
        onReadMessage={enabled ? markRead : undefined}
        onReplyMessage={enabled && !disabledReason ? setReplyTo : undefined}
        readThroughMessageID={lastReadMessageID}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={(draft) => {
          if (jobs.some((job) => job.id === draft.clientMessageID)) queue?.retry(draft.clientMessageID)
          else setRetryDraft(draft)
        }}
        attachmentRetryDisabled={Boolean(disabledReason)}
        retryFailedMessageDisabled={
          Boolean(disabledReason) ||
          outgoing.messages.some((message) => message.status === "sending")
        }
      />
      <ConversationComposer
        attachmentTargetIdentityID={!conversationID ? peerIdentityID : undefined}
        attachmentAgentDraft={attachmentAgentIdentityID
          ? { conversationID, agentIdentityID: attachmentAgentIdentityID }
          : undefined}
        onAttachmentConversationCreated={onAttachmentConversationCreated}
        onBeforeSend={() => prepareSendRef.current?.() ?? Promise.resolve(true)}
        conversationID={conversationID}
        conversationType={conversationType}
        retryFailedMessage
        disabledReason={disabledReason}
        retryDraft={retryDraft}
        replyTo={replyTo}
        onRetryDraftHandled={() => setRetryDraft(null)}
        onReplyToChange={setReplyTo}
        sendIndividualMessage={sendIndividualMessage}
        onSucceeded={() => void invalidate(resourceKeys.inbox())}
        onSending={outgoing.start}
        onSent={outgoing.succeed}
        onFailed={outgoing.fail}
      />
    </>
  )
}
