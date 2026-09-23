/** 移动端真人单聊与 AI 聊天详情。 */
import { SearchIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useNavigate, useParams } from "react-router"

import {
  ConversationType,
  isAgentInboxConversation,
  isDirectInboxConversation,
  type AgentInboxConversationData,
  type DirectInboxConversationData,
} from "@/api"
import { MobileIndividualThread } from "@/apps/mobile/mobile-individual-thread"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import {
  mobileSearchPath,
  useMobileNavigation,
  type MobileLocateState,
} from "@/apps/mobile/mobile-navigation"
import { Button } from "@/components/ui/button"
import { useConversationSummary } from "@/features/inbox/use-conversation-summary"
import { LoadingIndicator } from "@/components/loading-indicator"
import { useAccountDisabledReason } from "@/features/inbox/use-account-disabled-reason"
import { useConversationTypingLabel } from "@/features/inbox/use-conversation-typing"
type MobileIndividualLocationState = MobileLocateState & {
  memberUserID?: string
  conversation?: DirectInboxConversationData | AgentInboxConversationData
}

/** 展示当前双方会话的移动端头部和会话内搜索入口。 */
export function MobileIndividualHeader({
  conversation,
  peerName,
}: {
  conversation: DirectInboxConversationData | AgentInboxConversationData | null
  peerName: string
}) {
  const { t: tInbox } = useTranslation("inbox")
  const { chatsURL } = useMobileNavigation()
  const location = useLocation()
  const navigate = useNavigate()
  const { memberUserID } =
    (location.state as MobileIndividualLocationState | null) ?? {}
  const typingLabel = useConversationTypingLabel(conversation?.id ?? "", null)

  return (
    <MobilePageHeader
      backTo={
        memberUserID ? `/contacts/employees/${memberUserID}` : chatsURL
      }
      title={
        <span className="block min-w-0 truncate">{typingLabel || peerName}</span>
      }
      actions={
        <Button
          variant="ghost"
          size="icon-lg"
          className="-mr-2"
          aria-label={tInbox("searchCurrentConversation")}
          disabled={!conversation}
          onClick={() => {
            if (conversation)
              void navigate(mobileSearchPath(conversation.id), {
                state: { mobileBack: true },
              })
          }}
        >
          <SearchIcon />
        </Button>
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
  const { chatsURL } = useMobileNavigation()
  const { conversationID = "" } = useParams()
  const location = useLocation()
  const summary = useConversationSummary(conversationID, false)
  // 路由携带的摘要保持首屏线程，查询完成后由服务端结果接管。
  const initial = (location.state as MobileIndividualLocationState | null)?.conversation
  const data = summary.data === undefined && initial?.id === conversationID
    ? initial : summary.data
  const conversation = data && data.type === conversationType &&
    (isDirectInboxConversation(data) || isAgentInboxConversation(data)) ? data : null
  const disabledReason = useAccountDisabledReason(conversation)
  if (!conversationID) return <Navigate to={chatsURL} replace />
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
          {summary.error ? (
            <Button
              variant="outline"
              className="min-h-11"
              onClick={() => void summary.refresh()}
            >
              {t("common:actions.retry")}
            </Button>
          ) : null}
        </div>
      ) : (
        <MobileIndividualThread
          key={conversationID}
          conversationID={conversationID}
          conversationType={conversationType}
          peerIdentityID={conversation.direct?.peerIdentityId ?? ""}
          disabledReason={disabledReason}
          lastReadMessageID={conversation.lastReadMessageId}
          locateMessage={(location.state as MobileIndividualLocationState | null)?.locateMessage}
        />
      )}
    </section>
  )
}
