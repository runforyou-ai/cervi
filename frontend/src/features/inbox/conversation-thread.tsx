/** 会话消息线程与回复区的即时消息协调。 */
import { useCallback, useEffect, useEffectEvent, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  ChannelType,
  ConversationType,
  isAgentInboxConversation,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  markConversationRead,
  sendFirstAgentTextMessage,
  sendFirstDirectTextMessage,
  updateConversationUnreadMark,
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
import { useAttachmentQueue } from "@/features/inbox/attachment-queue-context"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import { useOutgoingMessages } from "@/features/inbox/outgoing-message-context"
import type { OutgoingConversationDraft } from "@/features/inbox/outgoing-message-store"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"

/** 连接时间线与回复区，处理已读、发送和草稿转正会话。 */
export function ConversationThread({
  agentDraftID,
  conversation,
  directTarget,
  groupParticipants,
  replyDisabledReason,
  onConversationChanged,
  onChatStarted,
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
}) {
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const pageActive = usePortalContainer()?.active ?? true
  const { identity } = useWorkspace()
  const { queue: attachmentQueue, jobs: attachmentJobs } = useAttachmentQueue()
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

  /** 清除未读标记失败时记录日志并恢复会话入口。 */
  const handleUnreadMarkClearError = useEffectEvent(
    (id: string, error: unknown) => {
      console.warn("清除会话未读标记失败", { conversationId: id, error })
      recoverSession(error, navigate)
    },
  )

  useEffect(() => {
    if (
      !conversationID ||
      !pageActive ||
      conversationType === ConversationType.ConversationTypeCustomer
    )
      return
    let current = true
    // 每次进入会话时清除服务端未读标记。
    void updateConversationUnreadMark(conversationID, { markedUnread: false })
      .then(() => invalidate(resourceKeys.inbox()))
      .catch((error: unknown) => {
        if (!current) return
        handleUnreadMarkClearError(conversationID, error)
      })
    return () => {
      current = false
    }
  }, [conversationID, conversationType, pageActive, invalidate])

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
  const messageSending = outgoing.messages.some(
    (message) => message.status === "sending",
  )
  const replySupported =
    !conversation ||
    isAgentInboxConversation(conversation) ||
    isDirectInboxConversation(conversation) ||
    isGroupInboxConversation(conversation) ||
    (conversation.customer.channelType === ChannelType.ChannelTypeWebsite ||
      conversation.customer.channelType === ChannelType.ChannelTypeTelegram)
  const telegramConversation = Boolean(
    conversation &&
    isCustomerInboxConversation(conversation) &&
    conversation.customer.channelType === ChannelType.ChannelTypeTelegram,
  )
  const groupConversation =
    conversation && isGroupInboxConversation(conversation) ? conversation : null

  /** 保存当前已看到的最新消息并刷新收件箱未读摘要。 */
  const markRead = useCallback(
    (messageID: string) => {
      if (!conversation) return
      void markConversationRead(conversation.id, {
        lastReadMessageId: messageID,
        clearUnreadMark: false,
      })
        .then(() => {
          void invalidate(resourceKeys.inbox())
          void invalidate(resourceKeys.conversationSummary(conversation.id))
        })
        .catch((error: unknown) =>
          console.warn("标记会话已读失败", {
            conversationId: conversation.id,
            error,
          }),
        )
    },
    [conversation, invalidate],
  )

  return (
    <>
      <ConversationTimeline
        prepareSendRef={prepareSendRef}
        customerDeliveries={telegramConversation}
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        // 合并发送中的文本消息与当前会话的附件任务。
        outgoingMessages={[...outgoing.messages, ...attachmentJobs.filter(job => job.stage !== "cancelled" && (conversationID ? job.conversationID === conversationID : directTarget && job.targetIdentityID === directTarget.id)).map(job => job.message)]}
        onRetryFailedMessage={(draft) => {
          if (attachmentJobs.some(job => job.id === draft.clientMessageID)) attachmentQueue?.retry(draft.clientMessageID)
          else setRetryDraft(draft)
        }}
        retryFailedMessageDisabled={
          messageSending || !replySupported || Boolean(replyDisabledReason)
        }
        groupParticipants={groupParticipants}
        onReplyMessage={
          conversation && replySupported && !replyDisabledReason
            ? setReplyTo
            : undefined
        }
        onReadMessage={conversation ? markRead : undefined}
        readThroughMessageID={conversation?.lastReadMessageId}
        enabled={Boolean(conversation)}
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
        onAttachmentConversationCreated={(created) => {
          if (directTarget) void invalidate(resourceKeys.directConversation(directTarget.id))
          void invalidate(resourceKeys.conversationMessages(created.id))
          if (aliveRef.current && isDirectInboxConversation(created)) onChatStarted(created)
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
