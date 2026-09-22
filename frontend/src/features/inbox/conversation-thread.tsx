/** 会话消息线程与回复区的即时消息协调。 */
import { useEffect, useRef, type RefObject } from "react"
import { useTranslation } from "react-i18next"

import {
  ChannelType,
  ConversationType,
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type AgentInboxConversationData,
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
import { listAllMemberOptions } from "@/features/inbox/list-all-member-options"
import { useConversationReadMarker } from "@/features/inbox/use-conversation-read-marker"
import { useFirstChatMessage } from "@/features/inbox/use-first-chat-message"
import { useThreadComposerBridge } from "@/features/inbox/use-thread-composer-bridge"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

/** 连接时间线与回复区，处理已读、发送和草稿转正会话。 */
export function ConversationThread({
  agentDraftID,
  draftWorkspaceID = "",
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
  /** AI 聊天草稿选定的本机工作区，首次发送时随消息一起绑定。 */
  draftWorkspaceID?: string
  replyDisabledReason: string | null
  onConversationChanged: () => void
  onChatStarted?: (
    conversation: DirectInboxConversationData | AgentInboxConversationData,
  ) => void
  locateMessage: ConversationLocateTarget | null
  customerDraftRef?: RefObject<ComposerDraftBridge | null>
}) {
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
  const bridge = useThreadComposerBridge(
    conversationID || agentDraftID,
    directPeerIdentityID ? `draft:${directPeerIdentityID}` : "",
  )
  const firstChat = useFirstChatMessage()
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
  // 客户会话的附件入口按渠道外发能力开放。
  const customerAttachment = customerConversation?.customer ?? null
  // 客户会话的内部备注可以提醒企业成员。
  const noteMentionMembers = useResource(resourceKeys.memberOptions(), listAllMemberOptions, {
    enabled: Boolean(customerConversation),
  })

  return (
    <>
      <ConversationTimeline
        {...bridge.timeline}
        customerDeliveries={telegramConversation}
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        retryFailedMessageDisabled={customerReplyUnavailable}
        onReplyMessage={
          conversation &&
          ((replySupported && !replyDisabledReason) || Boolean(customerConversation))
            ? bridge.selectReplyTarget
            : undefined
        }
        noteReplyEnabled={Boolean(customerConversation)}
        customerReplyUnavailable={customerReplyUnavailable}
        replyVisibility={bridge.visibility}
        onReadMessage={conversation ? markRead : undefined}
        readThroughMessageID={conversation?.lastReadMessageId}
        enabled={Boolean(conversation)}
        locateMessage={locateMessage}
      />
      <ConversationComposer
        {...bridge.composer}
        disabledReason={!replySupported ? t("channelReplyUnsupported") : replyDisabledReason}
        conversationID={conversationID}
        conversationType={conversationType}
        submitOnEnter
        refocusAfterSubmit
        groupParticipants={groupParticipants}
        noteMentionMembers={noteMentionMembers.data}
        currentIdentityID={identity.user.identityId}
        onVisibilityChange={customerConversation ? bridge.setVisibility : undefined}
        onFailed={(clientMessageID) => {
          bridge.outgoing.fail(clientMessageID)
          if (conversationID) void invalidate(resourceKeys.conversationSummary(conversationID))
          // 发送失败后刷新群资料。
          if (groupConversation)
            void invalidate(resourceKeys.groupConversation(groupConversation.id))
        }}
        onSucceeded={onConversationChanged}
        customerChannel={customerAttachment}
        attachmentTargetIdentityID={directTarget && !agentDraftID ? directTarget.id : undefined}
        attachmentAgentDraft={directTarget && agentDraftID ? { conversationID: agentDraftID, agentIdentityID: directTarget.id, workspaceID: draftWorkspaceID } : undefined}
        draftBridgeRef={customerDraftRef}
        onAttachmentConversationCreated={(created) => {
          if (!created) return
          firstChat.refreshStarted(created, directTarget && !agentDraftID ? directTarget.id : undefined)
          // 附件首发成功后切到新建的单聊或 AI 聊天。
          if (aliveRef.current && (isDirectInboxConversation(created) || isAgentInboxConversation(created))) onChatStarted?.(created)
        }}
        sendIndividualMessage={
          directTarget
            ? async (input) => {
                const result = agentDraftID
                  ? await firstChat.sendAgent(agentDraftID, directTarget.id, input, draftWorkspaceID)
                  : await firstChat.sendDirect(directTarget.id, input)
                // 首条发送成功后切到新建会话；线程已卸载时只刷新列表。
                if (aliveRef.current) {
                  onChatStarted?.(result.conversation)
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
