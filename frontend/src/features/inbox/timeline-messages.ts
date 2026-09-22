/** 会话时间线的消息模型：合并服务端窗口与即时发送项，并提供发送者与提醒名称。 */
import {
  ChatSubjectKind,
  MessageType,
  type ConversationMessageData,
} from "@/api"
import {
  coveredByWindow,
  windowCoverage,
  type OutgoingConversationDraft,
  type OutgoingConversationMessage,
} from "@/features/inbox/outgoing-message-store"
import { resources, supportedLanguages } from "@/i18n/resources"

import { compareConversationMessages } from "./conversation-window"

/** 时间线中的一条消息，本地发送项与服务端消息共用该结构。 */
export type TimelineMessage = Pick<
  ConversationMessageData,
  | "id"
  | "type"
  | "visibility"
  | "body"
  | "attachment"
  | "originatedAt"
  | "sender"
  | "sessionStart"
  | "systemEvent"
  | "replyTo"
  | "canReply"
  | "canNoteReply"
  | "mentions"
  | "mentionAll"
  | "agentProcess"
> & {
  persistedMessageID: string | null
  clientMessageID: string | null
  draftMentions: OutgoingConversationDraft["mentions"]
  mentionAllToken: OutgoingConversationDraft["mentionAllToken"]
  local: boolean
  deliveryStatus: "sending" | "failed" | null
}

const mentionAllNames = supportedLanguages.map(
  (language) => resources[language].inbox.messageMentionAll,
)

/** 返回视觉分组使用的稳定发送者标识。 */
export function timelineSenderKey(
  message: TimelineMessage,
  currentIdentityID: string,
) {
  if (message.local) {
    return `${ChatSubjectKind.ChatSubjectKindOrganizationIdentity}:${currentIdentityID}`
  }
  if (!message.sender) return `unknown:${message.id}`
  return `${message.sender.kind}:${message.sender.sourceId}`
}

/** 合并服务端消息和当前页面的即时发送状态。 */
export function mergeTimelineMessages(
  current: ConversationMessageData[],
  outgoing: OutgoingConversationMessage[],
) {
  const messages: TimelineMessage[] = [...current].sort(compareConversationMessages).map((message) => ({
    ...message,
    persistedMessageID: message.id,
    clientMessageID: null,
    draftMentions: [],
    mentionAllToken: null,
    local: false,
    deliveryStatus: null,
  }))
  const coverage = windowCoverage(current)
  for (const message of outgoing) {
    // 只为窗口之外的发送项生成本地气泡。
    if (coveredByWindow(message, coverage)) continue
    messages.push({
      id: `local:${message.clientMessageID}`,
      persistedMessageID: message.saved?.id ?? null,
      type: message.saved?.type ?? (message.attachment ? MessageType.MessageTypeAttachment : MessageType.MessageTypeText),
      visibility: message.saved?.visibility ?? message.visibility,
      attachment: message.saved?.attachment ?? message.attachment ?? null,
      body: message.body,
      originatedAt: message.originatedAt,
      sender: null,
      agentProcess: null,
      sessionStart: null,
      systemEvent: null,
      replyTo: message.replyTo,
      canReply: false,
      canNoteReply: false,
      mentions: message.mentions.map((mention) => ({
        chatSubjectId: mention.chatSubjectID ?? "",
        kind: ChatSubjectKind.ChatSubjectKindOrganizationIdentity,
        sourceId: mention.identityID,
        displayName: mention.displayName,
      })),
      clientMessageID: message.clientMessageID,
      draftMentions: message.mentions,
      mentionAll: message.mentionAll,
      mentionAllToken: message.mentionAllToken,
      local: true,
      deliveryStatus:
        message.status === "failed"
          ? "failed"
          : message.showSending && !message.saved
            ? "sending"
            : null,
    })
  }
  // 服务端消息只来自连续窗口，尚未补入窗口的发送结果继续作为本地项目展示。
  return messages
}

/** 收集一条消息中结构化提醒的成员名称，长名称优先匹配。 */
export function messageMentionNames(message: TimelineMessage) {
  return [
    ...message.mentions.map((mention) => mention.displayName?.trim() ?? ""),
    ...(message.mentionAll ? mentionAllNames : []),
  ]
    .filter((name, index, values) => name && values.indexOf(name) === index)
    .sort((left, right) => right.length - left.length)
}
