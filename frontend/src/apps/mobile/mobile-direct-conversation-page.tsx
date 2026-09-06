/** 移动端企业成员内部单聊详情。 */
import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useParams } from "react-router"

import {
  CustomerInboxView,
  InboxScope,
  isDirectInboxConversation,
  loadInbox,
  type DirectInboxConversationData,
  type InboxConversation,
} from "@/api"
import { MobileDirectThread } from "@/apps/mobile/mobile-direct-thread"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import { LoadingIndicator } from "@/components/loading-indicator"
import { ProfileAvatar } from "@/components/profile-avatar"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

const directInboxQuery = {
  scope: InboxScope.InboxScopeInternal,
  customerView: CustomerInboxView.CustomerInboxViewQueue,
  assigneeIdentityId: "",
}

type MobileDirectLocationState = {
  conversation?: InboxConversation
  memberUserID?: string
}

/** 展示当前 Direct 的移动端头部。 */
function MobileDirectHeader({
  conversation,
  peerName,
}: {
  conversation: DirectInboxConversationData | null
  peerName: string
}) {
  const { t: tInbox } = useTranslation("inbox")
  const { inboxURL } = useMobileNavigation()
  const location = useLocation()
  const memberUserID = (location.state as MobileDirectLocationState | null)
    ?.memberUserID
  const agentRunLabel = agentRunStatusLabel(
    conversation?.direct.agentRunStatus ?? null,
    tInbox,
  )

  return (
    <MobilePageHeader
      backTo={memberUserID ? `/contacts/employees/${memberUserID}` : inboxURL}
      title={
        <span className="flex items-center gap-3">
          {conversation ? (
            <ConversationAvatar conversation={conversation} className="size-9" />
          ) : (
            <ProfileAvatar name={peerName} className="size-9" />
          )}
          <span className="min-w-0 flex-1">
            <span className="block truncate text-base font-semibold">
              {peerName}
            </span>
            {agentRunLabel ? (
              <span className="block text-xs font-normal text-muted-foreground">
                {agentRunLabel}
              </span>
            ) : null}
          </span>
        </span>
      }
    />
  )
}

/** 加载并显示移动端 Direct 历史和文本发送区。 */
export function MobileDirectConversationPage() {
  const { t } = useTranslation("inbox")
  const { inboxURL } = useMobileNavigation()
  const location = useLocation()
  const { conversationID = "" } = useParams()
  const stateConversation = useMemo(() => {
    // 从路由状态读取本次打开的 Direct 摘要。
    const candidate = (location.state as MobileDirectLocationState | null)
      ?.conversation
    return candidate?.id === conversationID &&
      isDirectInboxConversation(candidate)
      ? candidate
      : null
  }, [conversationID, location.state])
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const { data, loading } = useResource(
    resourceKeys.inbox(directInboxQuery),
    () => loadInbox(directInboxQuery),
    {
      staleTime: 0,
      refetchInterval: pollingActive ? memberChatPollingInterval : false,
      refetchOnWindowFocus: false,
    },
  )

  if (!conversationID) return <Navigate to={inboxURL} replace />

  const matchedConversation = data?.conversations.find(
    (conversation) => conversation.id === conversationID,
  )
  if (matchedConversation && !isDirectInboxConversation(matchedConversation)) {
    return <Navigate to={inboxURL} replace />
  }
  const conversation =
    (matchedConversation && isDirectInboxConversation(matchedConversation)
      ? matchedConversation
      : null) ?? stateConversation
  const peerName = conversation?.direct.peerName.trim() || t("unknownSender")

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobileDirectHeader conversation={conversation} peerName={peerName} />
      {loading && !conversation ? (
        <LoadingIndicator className="min-h-0 flex-1 justify-center">
          {t("messagesLoading")}
        </LoadingIndicator>
      ) : (
        <MobileDirectThread key={conversationID} conversationID={conversationID} />
      )}
    </section>
  )
}
