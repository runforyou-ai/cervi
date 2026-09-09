/** 为 AI 新对话分配稳定路由，在同一页面内完成草稿和正式会话的交接。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useNavigate, useParams } from "react-router"

import {
  ConversationType,
  getAgent,
  isAgentInboxConversation,
  isNotFoundApiError,
  sendFirstAgentTextMessage,
  UserStatus,
  type AgentData,
  type AgentInboxConversationData,
} from "@/api"
import { MobileIndividualHeader } from "@/apps/mobile/mobile-individual-conversation-page"
import { MobileIndividualThread } from "@/apps/mobile/mobile-individual-thread"
import { MobilePageState } from "@/apps/mobile/mobile-page"
import { LoadingIndicator } from "@/components/loading-indicator"
import { useConversationSummary } from "@/features/inbox/use-conversation-summary"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

type MobileAgentLocationState = {
  draftAgentID?: string
  mobileBack?: boolean
  agentDirectory?: boolean
}

/** 打开草稿前确定会话地址，首发成功时无需切换页面。 */
export function MobileAgentChatPage() {
  const { agentID = "" } = useParams()
  const location = useLocation()
  const [conversationID] = useState(() => crypto.randomUUID())
  return (
    <Navigate
      to={`/inbox/agent/${conversationID}`}
      replace
      state={{ ...location.state, draftAgentID: agentID, agentDirectory: true }}
    />
  )
}

/** 按会话编号隔离草稿，首发前后保留同一个聊天实例。 */
export function MobileAgentConversationPage() {
  const { conversationID = "" } = useParams()
  return (
    <MobileAgentConversation
      key={conversationID}
      conversationID={conversationID}
    />
  )
}

/** 首发后启用正式查询，保留已发送气泡和输入框直至服务端数据接管。 */
function MobileAgentConversation({ conversationID }: { conversationID: string }) {
  const { t } = useTranslation(["mobile", "inbox", "common"])
  const navigate = useNavigate()
  const location = useLocation()
  const [draftAgentID] = useState(
    () => (location.state as MobileAgentLocationState | null)?.draftAgentID ?? "",
  )
  const [draftAgent, setDraftAgent] = useState<AgentData | null>(null)
  const [created, setCreated] = useState<AgentInboxConversationData | null>(null)
  const persisted = !draftAgentID || Boolean(created)
  const alive = useRef(true)
  const agent = useResource(
    resourceKeys.agent(draftAgentID),
    () => getAgent(draftAgentID),
    { staleTime: 0, enabled: !persisted && !draftAgent },
  )
  const summary = useConversationSummary(persisted ? conversationID : "", false)
  // 首发结果只用于当前页面过渡；后续摘要（包括不可用结果）由查询接管。
  const conversation =
    summary.data === undefined
      ? created
      : summary.data && isAgentInboxConversation(summary.data)
        ? summary.data
        : null
  const resource = persisted ? summary : agent
  const ready = persisted ? Boolean(conversation) : Boolean(draftAgent)

  useEffect(() => {
    // 完成本次账号校验后固定草稿目标，后续查询变化不替换输入区。
    if (
      !draftAgent &&
      agent.data?.status === UserStatus.UserStatusActive &&
      !agent.refreshing &&
      !agent.error
    ) setDraftAgent(agent.data)
  }, [draftAgent, agent.data, agent.refreshing, agent.error])
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobileIndividualHeader
        conversation={conversation}
        peerName={
          conversation?.agent.title ?? draftAgent?.displayName ??
          agent.data?.displayName ?? t("contacts.agents")
        }
      />
      {ready ? (
        <MobileIndividualThread
          conversationID={conversationID}
          conversationType={ConversationType.ConversationTypeAgent}
          enabled={persisted}
          sendIndividualMessage={!persisted && draftAgent ? async (input) => {
            console.info("发起移动端 AI 会话", {
              conversationId: conversationID,
              agentIdentityId: draftAgent.identityId,
              clientMessageId: input.clientMessageId,
            })
            const result = await sendFirstAgentTextMessage({
              conversationId: conversationID,
              agentIdentityId: draftAgent.identityId,
              clientMessageId: input.clientMessageId,
              body: input.body,
            })
            if (alive.current) {
              setCreated(result.conversation)
              // 只清除草稿标记，刷新地址时直接读取已保存的会话。
              void navigate(`/inbox/agent/${conversationID}`, {
                replace: true,
                state: { ...location.state, draftAgentID: undefined },
              })
            }
            return result.message
          } : undefined}
        />
      ) : resource.loading || resource.refreshing || (!persisted && agent.data?.status === UserStatus.UserStatusActive && !agent.error) ? (
        <LoadingIndicator className="min-h-0 flex-1 justify-center">
          {t("common:status.loading")}
        </LoadingIndicator>
      ) : (
        <MobilePageState
          title={persisted
            ? t(resource.error
              ? "inbox:conversationLoadError" : "inbox:conversationUnavailable")
            : t(resource.error && !isNotFoundApiError(resource.error)
              ? "agents.chatError" : "agents.unavailable")}
          onRetry={resource.error ? () => void resource.refresh() : undefined}
        />
      )}
    </section>
  )
}
