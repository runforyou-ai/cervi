/** 消息页会话主区，组合会话头、消息线程和联系人上下文栏。 */
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import {
  ConversationStatus,
  ServiceSessionStatus,
  getGroupConversation,
  isAgentInboxConversation,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type AgentInboxConversationData,
  type DirectInboxConversationData,
} from "@/api"
import { useWorkspace } from "@/contexts/workspace-context"
import { ConversationContextPane } from "@/features/inbox/conversation-context-pane"
import { ConversationHeader } from "@/features/inbox/conversation-header"
import { ConversationThread } from "@/features/inbox/conversation-thread"
import { DirectConversationDraftHeader } from "@/features/inbox/direct-conversation-draft-header"
import { sessionStatusLabel } from "@/features/inbox/session-status-label"
import type { ConversationSelection } from "@/features/inbox/inbox-selection"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { resourceKeys } from "@/hooks/resource-keys"
import { useIsWideViewport } from "@/hooks/use-narrow-viewport"
import { useResource } from "@/hooks/use-resource"

/** 按当前选择渲染会话头、消息线程和联系人上下文栏。 */
export function ConversationMain({
  selection,
  onSessionChanged,
  onConversationChanged,
  onGroupLeft,
  onChatStarted,
  narrowViewport = false,
}: {
  selection: ConversationSelection
  onSessionChanged: (conversationID: string) => void
  onConversationChanged: (conversationID: string) => void
  onGroupLeft: (conversationID: string) => void
  onChatStarted: (
    conversation: DirectInboxConversationData | AgentInboxConversationData,
  ) => void
  narrowViewport?: boolean
}) {
  const conversation =
    selection.kind === "conversation" ? selection.conversation : null
  const directTarget =
    selection.kind !== "conversation" ? selection.member : null
  const { t } = useTranslation("inbox")
  const { identity } = useWorkspace()
  const isWideViewport = useIsWideViewport()
  const conversationName = useConversationName()
  const [contextCollapsed, setContextCollapsed] = useState(
    () => !isWideViewport,
  )

  useEffect(() => {
    // 跨过响应式断点时恢复当前宽度对应的默认状态。
    setContextCollapsed(!isWideViewport)
  }, [isWideViewport])

  const sourceGroupConversation =
    conversation && isGroupInboxConversation(conversation) ? conversation : null
  const groupPollingActive = useMemberChatPollingActive()
  const groupResource = useResource(
    resourceKeys.groupConversation(sourceGroupConversation?.id ?? ""),
    () => getGroupConversation(sourceGroupConversation?.id ?? ""),
    {
      enabled: Boolean(sourceGroupConversation),
      staleTime: 0,
      refetchInterval: groupPollingActive ? memberChatPollingInterval : false,
    },
  )
  const group = groupResource.data
  // 用群资料的标题、头像、人数和状态覆盖列表摘要。
  const displayedConversation =
    sourceGroupConversation && group
      ? {
          ...sourceGroupConversation,
          group: {
            ...sourceGroupConversation.group,
            title: group.title,
            imageUrl: group.imageUrl,
            memberCount: group.participants.length,
            status: group.status,
          },
        }
      : conversation
  const contactName = displayedConversation
    ? conversationName(displayedConversation)
    : (directTarget?.displayName ?? "")
  const customerConversation =
    displayedConversation && isCustomerInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const directConversation =
    displayedConversation && isDirectInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const groupConversation =
    displayedConversation && isGroupInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const sessionStatus = customerConversation
    ? sessionStatusLabel(customerConversation.customer.serviceSessionStatus, t)
    : ""
  const replyDisabledReason = customerConversation
    ? customerConversation.customer.serviceSessionStatus ===
      ServiceSessionStatus.ServiceSessionStatusClosed
      ? t("replyClosedUnavailable")
      : customerConversation.customer.assignee &&
          customerConversation.customer.assignee.identityId !==
            identity.user.identityId
        ? t("replyAssignedUnavailable", {
            name: customerConversation.customer.assignee.displayName,
          })
        : null
    : groupConversation?.group.status ===
        ConversationStatus.ConversationStatusArchived
      ? t("groupDissolvedUnavailable")
      : null
  const validConversation =
    customerConversation ??
    directConversation ??
    groupConversation ??
    (displayedConversation && isAgentInboxConversation(displayedConversation)
      ? displayedConversation
      : null)
  if (!validConversation && !directTarget) return null
  // 线程键取 AI 草稿编号、真人草稿或单聊对端身份，其余取会话编号。
  const threadKey =
    selection.kind === "agent-draft"
      ? selection.conversationId
      : (directTarget?.id ??
        directConversation?.direct.peerIdentityId ??
        validConversation?.id)

  return (
    <div className="flex h-full min-h-0 bg-background">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        {validConversation ? (
          <ConversationHeader
            conversation={validConversation}
            contactName={contactName}
            sessionStatus={sessionStatus}
            currentIdentityId={identity.user.identityId}
            onSessionChanged={() => {
              if (customerConversation) onSessionChanged(customerConversation.id)
            }}
            narrowViewport={narrowViewport}
          />
        ) : directTarget ? (
          <DirectConversationDraftHeader member={directTarget} />
        ) : null}
        <ConversationThread
          key={threadKey}
          conversation={validConversation}
          directTarget={directTarget}
          groupParticipants={group?.participants}
          agentDraftID={
            selection.kind === "agent-draft" ? selection.conversationId : ""
          }
          replyDisabledReason={replyDisabledReason}
          onConversationChanged={() => {
            if (validConversation) onConversationChanged(validConversation.id)
          }}
          onChatStarted={onChatStarted}
        />
      </div>
      <ConversationContextPane
        conversation={validConversation}
        directTarget={directTarget}
        displayName={contactName}
        currentIdentityID={identity.user.identityId}
        onGroupLeft={() => {
          if (validConversation) onGroupLeft(validConversation.id)
        }}
        visible={!contextCollapsed}
        onToggle={() => setContextCollapsed((collapsed) => !collapsed)}
      />
    </div>
  )
}
