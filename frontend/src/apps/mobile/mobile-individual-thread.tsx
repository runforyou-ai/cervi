/** 移动端真人、AI 与客户聊天共用的时间线、阅读进度、文本与附件发送和失败重试。 */
import { useEffect, useRef, useState } from "react"

import {
  ConversationType,
  type ConversationMessageData,
  type ConversationMessageReference,
  type DirectTextMessageInput,
  type InboxConversation,
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

/** 草稿只展示本地发送状态，正式会话读取历史、推进已读、按需定位原消息并在前台轮询。 */
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
  customerDeliveries = false,
  locateMessage = null,
}: {
  conversationID: string
  conversationType?: ConversationType
  peerIdentityID?: string
  attachmentAgentIdentityID?: string
  onAttachmentConversationCreated?: (conversation: InboxConversation) => void
  enabled?: boolean
  disabledReason?: string | null
  lastReadMessageID?: string | null
  customerDeliveries?: boolean
  locateMessage?: ConversationLocateTarget | null
  sendIndividualMessage?: (
    input: DirectTextMessageInput,
  ) => Promise<ConversationMessageData>
}) {
  const { identity } = useMobileWorkspace()
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const invalidate = useResourceInvalidator()
  // 真人草稿尚无会话编号，发送状态按对端身份分组。
  const outgoing = useOutgoingMessages(
    conversationID,
    peerIdentityID ? `draft:${peerIdentityID}` : "",
  )
  // 客户会话没有手动未读标记，进入时只推进已读水位。
  const markRead = useConversationReadMarker(
    conversationID,
    enabled && conversationType !== ConversationType.ConversationTypeCustomer,
  )
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const [replyTo, setReplyTo] =
    useState<ConversationMessageReference | null>(null)
  const { record: recordRecentConversation } = useRecentConversations(
    identity.user.identityId,
  )

  useEffect(() => {
    // 正式会话打开后记入本机最近打开。
    if (conversationID && enabled) recordRecentConversation(conversationID)
  }, [conversationID, enabled, recordRecentConversation])

  return (
    <>
      <ConversationTimeline
        prepareSendRef={prepareSendRef}
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        requireWindowFocus={false}
        customerDeliveries={customerDeliveries}
        enabled={enabled}
        onReadMessage={enabled ? markRead : undefined}
        onReplyMessage={enabled && !disabledReason ? setReplyTo : undefined}
        readThroughMessageID={lastReadMessageID}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={setRetryDraft}
        retryFailedMessageDisabled={Boolean(disabledReason)}
        locateMessage={locateMessage}
      />
      <ConversationComposer
        attachmentTargetIdentityID={!conversationID ? peerIdentityID : undefined}
        attachmentAgentDraft={attachmentAgentIdentityID
          ? { conversationID, agentIdentityID: attachmentAgentIdentityID }
          : undefined}
        onAttachmentConversationCreated={(created) => {
          if (created) onAttachmentConversationCreated?.(created)
        }}
        onBeforeSend={() => prepareSendRef.current?.() ?? Promise.resolve(true)}
        conversationID={conversationID}
        conversationType={conversationType}
        currentIdentityID={identity.user.identityId}
        retryFailedMessage
        disabledReason={disabledReason}
        retryDraft={retryDraft}
        replyTo={replyTo}
        onRetryDraftHandled={() => setRetryDraft(null)}
        onReplyToChange={setReplyTo}
        sendIndividualMessage={sendIndividualMessage}
        onSucceeded={() => {
          void invalidate(resourceKeys.inbox())
          // 发送结果可能改变客服负责人与处理状态，同时刷新会话摘要。
          if (conversationID)
            void invalidate(resourceKeys.conversationSummary(conversationID))
        }}
        onSending={outgoing.start}
        onSent={outgoing.succeed}
        onFailed={outgoing.fail}
      />
    </>
  )
}
