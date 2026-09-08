/** 移动端真人单聊与 AI 聊天详情。 */
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useParams } from "react-router"

import {
  ConversationType,
  isAgentInboxConversation,
  isDirectInboxConversation,
  type AgentInboxConversationData,
  type DirectInboxConversationData,
} from "@/api"
import { MobileIndividualThread } from "@/apps/mobile/mobile-individual-thread"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import { Button } from "@/components/ui/button"
import { useConversationSummary } from "@/features/inbox/use-conversation-summary"
import { LoadingIndicator } from "@/components/loading-indicator"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
type MobileIndividualLocationState = { memberUserID?: string }

/** 展示当前双方会话的移动端头部。 */
function MobileIndividualHeader({
  conversation,
  peerName,
}: {
  conversation: DirectInboxConversationData | AgentInboxConversationData | null
  peerName: string
}) {
  const { t: tInbox } = useTranslation("inbox")
  const { inboxURL } = useMobileNavigation()
  const location = useLocation()
  const memberUserID = (location.state as MobileIndividualLocationState | null)
    ?.memberUserID
  const agentRunLabel = agentRunStatusLabel(
    conversation?.agent?.agentRunStatus ?? null,
    tInbox,
  )

  return (
    <MobilePageHeader
      backTo={memberUserID ? `/contacts/employees/${memberUserID}` : inboxURL}
      title={
        <span className="block min-w-0">
          <span className="block truncate">{peerName}</span>
          {conversation?.agent ? (
            <span className="block truncate text-xs font-normal text-muted-foreground">
              {conversation.agent.agentName}
              {agentRunLabel ? ` · ${agentRunLabel}` : ""}
            </span>
          ) : null}
        </span>
      }
    />
  )
}

/** 加载并显示移动端双方会话历史和文本发送区。 */
export function MobileIndividualConversationPage({
  conversationType,
}: {
  conversationType: ConversationType
}) {
  const { t } = useTranslation(["inbox", "common"])
  const { inboxURL } = useMobileNavigation()
  const { conversationID = "" } = useParams()
  const summary = useConversationSummary(conversationID, false)
  if (!conversationID) return <Navigate to={inboxURL} replace />
  const conversation = summary.data && summary.data.type === conversationType &&
    (isDirectInboxConversation(summary.data) || isAgentInboxConversation(summary.data)) ? summary.data : null
  const peerName =
    (conversation?.agent?.title ?? conversation?.direct?.peerName)?.trim() ||
    t("unknownSender")

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobileIndividualHeader conversation={conversation} peerName={peerName} />
      {summary.loading && !conversation ? (
        <LoadingIndicator className="min-h-0 flex-1 justify-center">
          {t("messagesLoading")}
        </LoadingIndicator>
      ) : !conversation ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
          <p>{t(summary.error ? "conversationLoadError" : "conversationUnavailable")}</p>
          {summary.error ? <Button variant="outline" size="sm" onClick={() => void summary.refresh()}>{t("common:actions.retry")}</Button> : null}
        </div>
      ) : (
        <MobileIndividualThread
          key={conversationID}
          conversationID={conversationID}
          conversationType={conversationType}
        />
      )}
    </section>
  )
}
