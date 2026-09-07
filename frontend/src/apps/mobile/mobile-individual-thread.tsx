/** 移动端真人与 AI 聊天共用的时间线、发送和失败重试。 */
import { useState } from "react"

import {
  ConversationType,
  type ConversationMessageData,
  type DirectTextMessageInput,
} from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import {
  useOutgoingConversationMessages,
  type OutgoingConversationDraft,
} from "@/features/inbox/use-outgoing-conversation-messages"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 草稿只展示本地发送状态，正式会话读取历史并在前台轮询。 */
export function MobileIndividualThread({
  conversationID,
  conversationType = ConversationType.ConversationTypeDirect,
  sendIndividualMessage,
}: {
  conversationID: string
  conversationType?: ConversationType
  sendIndividualMessage?: (
    input: DirectTextMessageInput,
  ) => Promise<ConversationMessageData>
}) {
  const { identity } = useMobileWorkspace()
  const invalidate = useResourceInvalidator()
  const outgoing = useOutgoingConversationMessages()
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)

  return (
    <>
      <ConversationTimeline
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        requireWindowFocus={false}
        enabled={Boolean(conversationID)}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={setRetryDraft}
        retryFailedMessageDisabled={outgoing.messages.some(
          (message) => message.status === "sending",
        )}
      />
      <ConversationComposer
        conversationID={conversationID}
        conversationType={conversationType}
        retryFailedMessage
        retryDraft={retryDraft}
        onRetryDraftHandled={() => setRetryDraft(null)}
        sendIndividualMessage={sendIndividualMessage}
        onSucceeded={() => void invalidate(resourceKeys.inbox())}
        onSending={outgoing.start}
        onSent={outgoing.succeed}
        onFailed={outgoing.fail}
      />
    </>
  )
}
