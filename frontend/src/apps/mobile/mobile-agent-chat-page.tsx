/** 为 AI 新对话分配稳定路由，在同一页面内完成草稿和正式会话的交接。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useNavigate, useParams } from "react-router"

import {
  ConversationType,
  getAgent,
  getAssistant,
  isAgentInboxConversation,
  isNotFoundApiError,
  UserStatus,
  type AgentData,
  type AgentInboxConversationData,
} from "@/api"
import { MobileIndividualHeader } from "@/apps/mobile/mobile-individual-conversation-page"
import { MobileIndividualThread } from "@/apps/mobile/mobile-individual-thread"
import type { MobileLocateState } from "@/apps/mobile/mobile-navigation"
import { MobilePageState } from "@/apps/mobile/mobile-page"
import { LoadingIndicator } from "@/components/loading-indicator"
import { useAccountDisabledReason } from "@/features/inbox/use-account-disabled-reason"
import { useConversationSummary } from "@/features/inbox/use-conversation-summary"
import { useFirstChatMessage } from "@/features/inbox/use-first-chat-message"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 移动端 AI 聊天草稿目标的身份、名称与账号状态。 */
type DraftTarget = Pick<AgentData, "identityId" | "displayName" | "status">

type MobileAgentLocationState = MobileLocateState & {
  draftAgentID?: string
  draftAssistant?: boolean
  mobileBack?: boolean
  agentDirectory?: boolean
}

/** 打开草稿前确定会话地址，首发成功后在当前页面展示会话。 */
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
  const [draftAssistant] = useState(
    () => (location.state as MobileAgentLocationState | null)?.draftAssistant ?? false,
  )
  const [draftAgent, setDraftAgent] = useState<DraftTarget | null>(null)
  const [created, setCreated] = useState<AgentInboxConversationData | null>(null)
  const persisted = !draftAgentID || Boolean(created)
  const alive = useRef(true)
  const firstChat = useFirstChatMessage()
  // 草稿目标为本人助理时读取助理详情，其余读取 AI 员工详情。
  const employee = useResource(
    resourceKeys.agent(draftAgentID),
    () => getAgent(draftAgentID),
    { staleTime: 0, enabled: !persisted && !draftAgent && !draftAssistant },
  )
  const assistant = useResource(
    resourceKeys.assistant(draftAgentID),
    () => getAssistant(draftAgentID),
    { staleTime: 0, enabled: !persisted && !draftAgent && draftAssistant },
  )
  const agent = draftAssistant
    ? { ...assistant, data: assistant.data?.assistant as DraftTarget | undefined }
    : employee
  const summary = useConversationSummary(persisted ? conversationID : "", false)
  // 首发结果只用于当前页面过渡；后续摘要（包括不可用结果）由查询接管。
  const conversation =
    summary.data === undefined
      ? created
      : summary.data && isAgentInboxConversation(summary.data)
        ? summary.data
        : null
  const disabledReason = useAccountDisabledReason(conversation)
  const resource = persisted ? summary : agent
  const ready = persisted ? Boolean(conversation) : Boolean(draftAgent)

  useEffect(() => {
    // 账号校验完成后固定草稿目标和输入区。
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

  /** 首发后保留聊天实例并清除路由中的草稿标记。 */
  function handleCreated(conversation: AgentInboxConversationData) {
    if (!alive.current) return
    setCreated(conversation)
    void navigate(`/inbox/agent/${conversationID}`, {
      replace: true,
      state: { ...location.state, draftAgentID: undefined },
    })
  }

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
          attachmentAgentIdentityID={!persisted ? draftAgent?.identityId : undefined}
          onAttachmentConversationCreated={(created) => {
            if (isAgentInboxConversation(created)) handleCreated(created)
          }}
          disabledReason={disabledReason}
          lastReadMessageID={conversation?.lastReadMessageId}
          locateMessage={(location.state as MobileAgentLocationState | null)?.locateMessage}
          sendIndividualMessage={!persisted && draftAgent ? async (input) => {
            const result = await firstChat.sendAgent(conversationID, draftAgent.identityId, input)
            handleCreated(result.conversation)
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
