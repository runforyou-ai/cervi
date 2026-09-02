/** 管理当前页面尚未确认和已经确认的成员发送消息。 */
import { useEffect, useRef, useState } from "react"

import type {
  ConversationMessageData,
  ConversationMessageReference,
} from "@/api"

export const conversationSendingIndicatorDelay = 300

export type OutgoingConversationDraft = {
  clientMessageID: string
  body: string
  originatedAt: string
  replyTo: ConversationMessageReference | null
  mentionSubjectIDs: string[]
}

export type OutgoingConversationMessage = OutgoingConversationDraft & {
  status: "sending" | "sent" | "failed"
  showSending: boolean
  saved: ConversationMessageData | null
}

/** 保存当前页面的即时发送状态，不执行重试。 */
export function useOutgoingConversationMessages() {
  const [messages, setMessages] = useState<OutgoingConversationMessage[]>([])
  const delayTimersRef = useRef(new Map<string, number>())

  useEffect(
    () => () => {
      for (const timer of delayTimersRef.current.values()) {
        window.clearTimeout(timer)
      }
      delayTimersRef.current.clear()
    },
    [],
  )

  /** 清除一条消息尚未触发的发送中提示。 */
  function clearSendingDelay(clientMessageID: string) {
    const timer = delayTimersRef.current.get(clientMessageID)
    if (timer === undefined) return
    window.clearTimeout(timer)
    delayTimersRef.current.delete(clientMessageID)
  }

  /** 立即展示一次新发送。 */
  function start(message: OutgoingConversationDraft) {
    setMessages((current) => {
      const next = {
        ...message,
        status: "sending" as const,
        showSending: false,
        saved: null,
      }
      return current.some(
        (item) => item.clientMessageID === message.clientMessageID,
      )
        ? current.map((item) =>
            item.clientMessageID === message.clientMessageID ? next : item,
          )
        : [...current, next]
    })
    const timer = window.setTimeout(() => {
      delayTimersRef.current.delete(message.clientMessageID)
      setMessages((current) =>
        current.map((item) =>
          item.clientMessageID === message.clientMessageID &&
          item.status === "sending"
            ? { ...item, showSending: true }
            : item,
        ),
      )
    }, conversationSendingIndicatorDelay)
    delayTimersRef.current.set(message.clientMessageID, timer)
  }

  /** 用服务端消息替换发送中的本地消息。 */
  function succeed(
    clientMessageID: string,
    saved: ConversationMessageData,
  ) {
    clearSendingDelay(clientMessageID)
    setMessages((current) =>
      current.map((message) =>
        message.clientMessageID === clientMessageID
          ? { ...message, status: "sent", saved }
          : message,
      ),
    )
  }

  /** 将本次发送原地标记为失败。 */
  function fail(clientMessageID: string) {
    clearSendingDelay(clientMessageID)
    setMessages((current) =>
      current.map((message) =>
        message.clientMessageID === clientMessageID
          ? { ...message, status: "failed", showSending: false }
          : message,
      ),
    )
  }

  return { messages, start, succeed, fail }
}
