/** 会话消息线程与回复区的即时消息协调。 */
import { useEffect, useRef, useState, type RefObject } from "react"
import { useTranslation } from "react-i18next"

import {
  ChannelType,
  ConversationType,
  MessageVisibility,
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
import {
  ConversationComposer,
  type ComposerDraftBridge,
} from "@/features/inbox/conversation-composer"
import {
  ConversationTimeline,
  type ConversationLocateTarget,
} from "@/features/inbox/conversation-timeline"
import { customerReplySupported } from "@/features/inbox/customer-session-actions"
import { useOutgoingMessages } from "@/features/inbox/outgoing-message-context"
import type { OutgoingConversationDraft } from "@/features/inbox/outgoing-message-store"
import { useConversationReadMarker } from "@/features/inbox/use-conversation-read-marker"
import { resourceKeys } from "@/hooks/resource-keys"
import { resolveAppPlatform } from "@/platform/app-platform"
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
  customerDraftRef,
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
  customerDraftRef?: RefObject<ComposerDraftBridge | null>
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
  const [visibility, setVisibility] = useState<MessageVisibility>(
    MessageVisibility.MessageVisibilityCustomerVisible,
  )
  // 对客回复与内部备注各自保留引用目标。
  const [replyTargets, setReplyTargets] = useState<
    Partial<Record<MessageVisibility, ConversationMessageReference | null>>
  >({})
  const replyTo = replyTargets[visibility] ?? null

  /** 按时间线给出的输入模式保存引用目标并切到该模式。 */
  function selectReplyTarget(
    message: ConversationMessageReference | null,
    target: MessageVisibility,
  ) {
    setVisibility(target)
    setReplyTargets((current) => ({ ...current, [target]: message }))
  }
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
  const customerConversation =
    conversation && isCustomerInboxConversation(conversation) ? conversation : null
  // 渠道不支持、周期已关闭或由他人负责时都不能对客回复。
  const customerReplyUnavailable = !replySupported || Boolean(replyDisabledReason)
  // 客户会话的附件入口按渠道外发能力开放，移动端留待独立交付。
  const customerAttachment = customerConversation?.customer ?? null
  const customerAttachmentSupported = Boolean(
    customerAttachment?.attachmentSupported && resolveAppPlatform() !== "mobile",
  )

  return (
    <>
      <ConversationTimeline
        prepareSendRef={prepareSendRef}
        customerDeliveries={telegramConversation}
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={(draft) => {
          setVisibility(draft.visibility)
          setReplyTargets((current) => ({ ...current, [draft.visibility]: draft.replyTo }))
          setRetryDraft(draft)
        }}
        retryFailedMessageDisabled={customerReplyUnavailable}
        groupParticipants={groupParticipants}
        onReplyMessage={
          conversation &&
          ((replySupported && !replyDisabledReason) || Boolean(customerConversation))
            ? selectReplyTarget
            : undefined
        }
        noteReplyEnabled={Boolean(customerConversation)}
        customerReplyUnavailable={customerReplyUnavailable}
        replyVisibility={visibility}
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
        onReplyToChange={(message) =>
          setReplyTargets((current) => ({ ...current, [visibility]: message }))
        }
        visibility={visibility}
        onVisibilityChange={customerConversation ? setVisibility : undefined}
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
        customerAttachmentSupported={customerAttachmentSupported}
        customerAttachmentByteLimit={customerAttachment?.attachmentByteLimit ?? 0}
        customerAttachmentCaptionLimit={customerAttachment?.attachmentCaptionLimit ?? 4000}
        attachmentTargetIdentityID={directTarget && !agentDraftID ? directTarget.id : undefined}
        attachmentAgentDraft={directTarget && agentDraftID ? { conversationID: agentDraftID, agentIdentityID: directTarget.id } : undefined}
        draftBridgeRef={customerDraftRef}
        onAttachmentConversationCreated={(created) => {
          if (!created) return
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
