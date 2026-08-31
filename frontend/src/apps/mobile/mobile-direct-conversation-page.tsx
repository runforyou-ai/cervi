/** 移动端企业成员内部单聊详情。 */
import { useMemo } from "react"
import {
  ArrowLeftIcon,
  BotIcon,
  UserRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Navigate,
  useLocation,
  useNavigate,
  useParams,
} from "react-router"

import {
  ConversationType,
  isDirectInboxConversation,
  loadInbox,
  OrganizationIdentityType,
  type DirectInboxConversationData,
  type InboxConversation,
} from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { Button } from "@/components/ui/button"
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

type MobileDirectLocationState = {
  conversation?: InboxConversation
}

/** 从路由状态读取本次打开的 Direct 摘要。 */
function directConversationFromState(
  state: unknown,
  conversationID: string,
): DirectInboxConversationData | null {
  const candidate = (state as MobileDirectLocationState | null)?.conversation
  return candidate?.id === conversationID &&
    isDirectInboxConversation(candidate)
    ? candidate
    : null
}

/** 展示当前 Direct 的移动端头部。 */
function MobileDirectHeader({
  conversation,
  peerName,
}: {
  conversation: DirectInboxConversationData | null
  peerName: string
}) {
  const { t } = useTranslation("common")
  const { t: tInbox } = useTranslation("inbox")
  const navigate = useNavigate()
  const initial = Array.from(peerName)[0]?.toLocaleUpperCase()
  const directAgent =
    conversation?.direct.peerType ===
    OrganizationIdentityType.OrganizationIdentityTypeAgent
  const agentRunLabel = agentRunStatusLabel(
    conversation?.direct.agentRunStatus ?? null,
    tInbox,
  )

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b px-2">
      <Button
        type="button"
        variant="ghost"
        size="icon-lg"
        aria-label={t("actions.back")}
        onClick={() => navigate("/inbox", { replace: true })}
      >
        <ArrowLeftIcon />
      </Button>
      <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-sm font-medium text-muted-foreground">
        {directAgent ? (
          <BotIcon className="size-4" />
        ) : initial ? (
          initial
        ) : (
          <UserRoundIcon className="size-4" />
        )}
      </span>
      <div className="min-w-0 flex-1">
        <h1 className="truncate text-base font-semibold">{peerName}</h1>
        {agentRunLabel ? (
          <p className="text-xs text-muted-foreground">{agentRunLabel}</p>
        ) : null}
      </div>
    </header>
  )
}

/** 加载并显示移动端 Direct 历史和文本发送区。 */
export function MobileDirectConversationPage() {
  const { t } = useTranslation("inbox")
  const { identity } = useMobileWorkspace()
  const invalidate = useResourceInvalidator()
  const location = useLocation()
  const { conversationID = "" } = useParams()
  const stateConversation = useMemo(
    () => directConversationFromState(location.state, conversationID),
    [conversationID, location.state],
  )
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const { data, loading } = useResource(
    resourceKeys.inbox(),
    () => loadInbox(),
    {
      staleTime: 0,
      refetchInterval: pollingActive ? memberChatPollingInterval : false,
      refetchOnWindowFocus: false,
    },
  )
  const outgoing = useOutgoingConversationMessages()

  if (!conversationID) return <Navigate to="/inbox" replace />

  const matchedConversation = data?.conversations.find(
    (conversation) => conversation.id === conversationID,
  )
  if (matchedConversation && !isDirectInboxConversation(matchedConversation)) {
    return <Navigate to="/inbox" replace />
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
            onSucceeded={() =>
              void invalidate(resourceKeys.inbox(), { exact: true })
            }
            onSending={outgoing.start}
            onSent={outgoing.succeed}
            onFailed={outgoing.fail}
          />
        </>
      )}
    </section>
  )
}
