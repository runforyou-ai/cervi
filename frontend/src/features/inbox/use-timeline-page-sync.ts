/** 把时间线窗口的读取结果同步给发送状态、正在输入状态和会话头的 AI 员工活动。 */
import { useEffect, useRef } from "react"

import { AgentRunStatus, type ConversationMessageListData } from "@/api"
import { publishConversationAgentActivity } from "@/features/inbox/conversation-agent-activity"
import {
  nextTypingArrival,
  type TypingArrivalBaseline,
} from "@/features/inbox/conversation-typing-arrival"
import { useOutgoingMessageStore } from "@/features/inbox/outgoing-message-context"
import { clearConversationTypingSender } from "@/features/inbox/use-conversation-typing"

/** 按当前窗口收敛发送项、清除新消息发送者的输入状态并发布 AI 员工活动。 */
export function useTimelinePageSync(
  conversationID: string,
  currentPage: ConversationMessageListData | null,
) {
  const outgoingStore = useOutgoingMessageStore()
  useEffect(() => {
    if (!currentPage) return
    // 窗口已收录的发送项从发送状态中删除。
    outgoingStore.reconcile(conversationID, currentPage.messages)
  }, [conversationID, currentPage, outgoingStore])
  const latestMessage = currentPage?.messages[currentPage.messages.length - 1]
  const latestMessageSeq = latestMessage?.messageSeq
  const latestSenderSubjectID = latestMessage?.sender?.chatSubjectId
  const windowLoaded = Boolean(currentPage)
  const typingArrivalRef = useRef<TypingArrivalBaseline>(null)
  useEffect(() => {
    // 新消息到达后不再显示其发送者正在输入；首次加载窗口与回看历史窗口都不算新消息。
    const arrival = nextTypingArrival(typingArrivalRef.current, {
      conversationID,
      loaded: windowLoaded,
      messageSeq: latestMessageSeq,
    })
    typingArrivalRef.current = arrival.baseline
    if (arrival.arrived && latestSenderSubjectID) clearConversationTypingSender(conversationID, latestSenderSubjectID)
  }, [conversationID, latestMessageSeq, latestSenderSubjectID, windowLoaded])
  const agentActivityNames = [
    ...(currentPage?.agentRuns ?? [])
      .filter((run) => run.status === AgentRunStatus.AgentRunStatusQueued || run.status === AgentRunStatus.AgentRunStatusRunning)
      .map((run) => run.agentName),
    ...(currentPage?.pendingAgents ?? []).map((agent) => agent.displayName),
  ].join("\u0000")
  useEffect(() => {
    // 会话头按时间线读到的运行与排队状态展示 AI 员工正在回复。
    publishConversationAgentActivity(conversationID, agentActivityNames ? agentActivityNames.split("\u0000") : [])
    return () => publishConversationAgentActivity(conversationID, [])
  }, [agentActivityNames, conversationID])
}
