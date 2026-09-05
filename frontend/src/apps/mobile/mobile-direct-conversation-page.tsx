/** 移动端企业成员内部单聊详情。 */
import { useMemo } from "react"
import { BotIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useParams } from "react-router"

import {
  ConversationType,
  CustomerInboxView,
  InboxScope,
  isDirectInboxConversation,
  loadInbox,
  OrganizationIdentityType,
  type DirectInboxConversationData,
  type InboxConversation,
} from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import { LoadingIndicator } from "@/components/loading-indicator"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { useOutgoingConversationMessages } from "@/features/inbox/use-outgoing-conversation-messages"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

const directInboxQuery = {
  scope: InboxScope.InboxScopeInternal,
  customerView: CustomerInboxView.CustomerInboxViewQueue,
  assigneeIdentityId: "",
}

type MobileDirectLocationState = {
  conversation?: InboxConversation
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
  const initial = Array.from(peerName)[0]?.toLocaleUpperCase()
  const directAgent =
    conversation?.direct.peerType ===
    OrganizationIdentityType.OrganizationIdentityTypeAgent
  const agentRunLabel = agentRunStatusLabel(
    conversation?.direct.agentRunStatus ?? null,
    tInbox,
  )

  return (
    <MobilePageHeader
      backTo={inboxURL}
      title={
        <span className="flex items-center gap-3">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-sm font-medium text-muted-foreground">
            {directAgent ? (
              <BotIcon className="size-4" />
            ) : initial ? (
              initial
            ) : (
              <UserRoundIcon className="size-4" />
            )}
          </span>
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
  const { identity } = useMobileWorkspace()
  const { inboxURL } = useMobileNavigation()
  const invalidate = useResourceInvalidator()
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
  const outgoing = useOutgoingConversationMessages()

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
        <>
          <ConversationTimeline
            conversationID={conversationID}
            conversationType={ConversationType.ConversationTypeDirect}
            currentIdentityID={identity.user.identityId}
            requireWindowFocus={false}
            outgoingMessages={outgoing.messages}
          />
          <ConversationComposer
            conversationID={conversationID}
            conversationType={ConversationType.ConversationTypeDirect}
            onSucceeded={() => void invalidate(resourceKeys.inbox())}
            onSending={outgoing.start}
            onSent={outgoing.succeed}
            onFailed={outgoing.fail}
          />
        </>
      )}
    </section>
  )
}
