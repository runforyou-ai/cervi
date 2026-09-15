/** 会话消息线程与回复区的即时消息协调。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import {
  ChannelType,
  ConversationType,
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  sendFirstAgentTextMessage,
  sendFirstDirectTextMessage,
  type AgentInboxConversationData,
  type ConversationMessageReference,
  type CustomerInboxConversationData,
  type DirectInboxConversationData,
  type GroupInboxConversationData,
  type GroupParticipant,
  type MemberOption,
} from "@/api"
import { usePortalContainer } from "@/components/ui/portal-container"
import { useWorkspace } from "@/contexts/workspace-context"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import {
  ConversationTimeline,
  type ConversationLocateTarget,
} from "@/features/inbox/conversation-timeline"
import { customerReplySupported } from "@/features/inbox/customer-session-actions"
import { useOutgoingMessages } from "@/features/inbox/outgoing-message-context"
import type { OutgoingConversationDraft } from "@/features/inbox/outgoing-message-store"
import { useConversationReadMarker } from "@/features/inbox/use-conversation-read-marker"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 连接时间线与回复区，处理已读、发送和草稿转正会话。 */
export function ConversationThread({
  agentDraftID,
  conversation,
  directTarget,
  groupParticipants,
  replyDisabledReason,
  onConversationChanged,
  onChatStarted,
  locateMessage,
}: {
  conversation:
    | CustomerInboxConversationData
    | AgentInboxConversationData
    | DirectInboxConversationData
    | GroupInboxConversationData
    | null
  directTarget: MemberOption | null
  groupParticipants: GroupParticipant[] | undefined
  agentDraftID: string
  replyDisabledReason: string | null
  onConversationChanged: () => void
  onChatStarted: (
    conversation: DirectInboxConversationData | AgentInboxConversationData,
  ) => void
  locateMessage: ConversationLocateTarget | null
}) {
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const { t } = useTranslation("inbox")
  const pageActive = usePortalContainer()?.active ?? true
  const { identity } = useWorkspace()
  const invalidate = useResourceInvalidator()
  const aliveRef = useRef(true)
  const conversationID = conversation?.id ?? ""
  // 真人草稿尚无会话编号，发送状态按对端身份分组。
  const directPeerIdentityID =
    directTarget?.id ??
    (conversation && isDirectInboxConversation(conversation)
      ? conversation.direct.peerIdentityId
      : "")
  const outgoing = useOutgoingMessages(
    conversationID || agentDraftID,
    directPeerIdentityID ? `draft:${directPeerIdentityID}` : "",
  )
  const conversationType =
    conversation?.type ??
    (agentDraftID
      ? ConversationType.ConversationTypeAgent
      : ConversationType.ConversationTypeDirect)
  const markRead = useConversationReadMarker(
    conversationID,
    pageActive && conversationType !== ConversationType.ConversationTypeCustomer,
  )

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
    }
  }, [])
  const [replyTo, setReplyTo] = useState<ConversationMessageReference | null>(
    null,
  )
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const replySupported =
    !conversation ||
    isAgentInboxConversation(conversation) ||
    isDirectInboxConversation(conversation) ||
    isGroupInboxConversation(conversation) ||
    customerReplySupported(conversation.customer)
  const telegramConversation = Boolean(
    conversation &&
    isCustomerInboxConversation(conversation) &&
    conversation.customer.channelType === ChannelType.ChannelTypeTelegram,
  )
  const groupConversation =
    conversation && isGroupInboxConversation(conversation) ? conversation : null

  return (
    <>
      <ConversationTimeline
        prepareSendRef={prepareSendRef}
        customerDeliveries={telegramConversation}
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={setRetryDraft}
        retryFailedMessageDisabled={!replySupported || Boolean(replyDisabledReason)}
        groupParticipants={groupParticipants}
        onReplyMessage={
          conversation && replySupported && !replyDisabledReason
            ? setReplyTo
            : undefined
        }
        onReadMessage={conversation ? markRead : undefined}
        readThroughMessageID={conversation?.lastReadMessageId}
        enabled={Boolean(conversation)}
        locateMessage={locateMessage}
      />
      <ConversationComposer
        disabledReason={!replySupported ? t("channelReplyUnsupported") : replyDisabledReason}
        onBeforeSend={() =>
          prepareSendRef.current?.() ?? Promise.resolve(true)
        }
        conversationID={conversationID}
        conversationType={conversationType}
        submitOnEnter
        refocusAfterSubmit
        retryFailedMessage
        retryDraft={retryDraft}
        replyTo={replyTo}
        groupParticipants={groupParticipants}
        currentIdentityID={identity.user.identityId}
        onRetryDraftHandled={() => setRetryDraft(null)}
        onReplyToChange={setReplyTo}
        onSending={outgoing.start}
        onSent={outgoing.succeed}
        onFailed={(clientMessageID) => {
          outgoing.fail(clientMessageID)
          if (conversationID) void invalidate(resourceKeys.conversationSummary(conversationID))
          // 发送失败后刷新群资料。
          if (groupConversation)
            void invalidate(resourceKeys.groupConversation(groupConversation.id))
        }}
        onSucceeded={onConversationChanged}
        attachmentTargetIdentityID={directTarget && !agentDraftID ? directTarget.id : undefined}
        attachmentAgentDraft={directTarget && agentDraftID ? { conversationID: agentDraftID, agentIdentityID: directTarget.id } : undefined}
        onAttachmentConversationCreated={(created) => {
          if (directTarget && !agentDraftID) void invalidate(resourceKeys.directConversation(directTarget.id))
          void invalidate(resourceKeys.conversationMessages(created.id))
          // 附件首发成功后切到新建的单聊或 AI 聊天。
          if (aliveRef.current && (isDirectInboxConversation(created) || isAgentInboxConversation(created))) onChatStarted(created)
        }}
        sendIndividualMessage={
          directTarget
            ? async (input) => {
                const result = agentDraftID
                  ? await sendFirstAgentTextMessage({
                      conversationId: agentDraftID,
                      agentIdentityId: directTarget.id,
                      clientMessageId: input.clientMessageId,
                      body: input.body,
                    })
                  : await sendFirstDirectTextMessage({
                      targetIdentityId: directTarget.id,
                      ...input,
                    })
                if (!agentDraftID)
                  void invalidate(
                    resourceKeys.directConversation(directTarget.id),
                  )
                void invalidate(
                  resourceKeys.conversationMessages(result.conversation.id),
                )
                // 首条发送成功后切到新建会话；线程已卸载时只刷新列表。
                if (aliveRef.current) {
                  onChatStarted(result.conversation)
                } else {
                  void invalidate(resourceKeys.inbox())
                }
                return result.message
              }
            : undefined
        }
      />
    </>
  )
}
