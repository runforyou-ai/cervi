/** 会话编辑器的输入参数与草稿桥接协议。 */
import type { RefObject } from "react"
import type {
  ConversationType, MessageVisibility, ConversationMessageData, CustomerInboxConversationData,
  ConversationMessageReference, DirectTextMessageInput, GroupParticipant, InboxConversationData, MemberOption,
} from "@/api"
import type { OutgoingConversationDraft } from "./outgoing-message-store"

/** 读取和替换回复输入框草稿的入口。 */
export type ComposerDraftBridge = {
  read: () => string
  replace: (body: string) => void
}

/** 客户会话所在渠道的附件与输入状态能力。 */
export type CustomerChannelCapabilities = Pick<
  CustomerInboxConversationData["customer"],
  "attachmentSupported" | "attachmentByteLimit" | "attachmentCaptionLimit" | "channelType"
>

/** 会话编辑器的调用参数。 */
export type ConversationComposerProps = {
  attachmentTargetIdentityID?: string
  attachmentAgentDraft?: { conversationID: string; agentIdentityID: string; customerConversationID?: string }
  customerChannel?: CustomerChannelCapabilities | null
  onAttachmentConversationCreated?: (conversation: InboxConversationData | null, conversationID: string) => void
  draftBridgeRef?: RefObject<ComposerDraftBridge | null>
  conversationID: string
  conversationType: ConversationType
  submitOnEnter?: boolean
  refocusAfterSubmit?: boolean
  disabledReason?: string | null
  visibility?: MessageVisibility
  onVisibilityChange?: (visibility: MessageVisibility) => void
  retryDraft?: OutgoingConversationDraft | null
  replyTo?: ConversationMessageReference | null
  groupParticipants?: GroupParticipant[]
  noteMentionMembers?: MemberOption[]
  currentIdentityID?: string
  onRetryDraftHandled?: () => void
  onReplyToChange?: (message: ConversationMessageReference | null) => void
  onBeforeSend?: () => Promise<boolean>
  onSending: (message: OutgoingConversationDraft) => void
  onSent: (clientMessageID: string, message: ConversationMessageData) => void
  onFailed: (clientMessageID: string) => void
  onSucceeded: () => void
  sendIndividualMessage?: (
    input: DirectTextMessageInput,
  ) => Promise<ConversationMessageData>
}
