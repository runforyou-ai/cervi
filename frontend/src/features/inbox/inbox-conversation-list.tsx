/** 消息页中栏的会话列表项和右键操作。 */
import { BellOffIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  ConversationStatus,
  MessageType,
  MessageVisibility,
  isAgentInboxConversation,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  isInternalInboxConversation,
  updateConversationUnreadMark,
  type InboxConversation,
} from "@/api"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import {
  ConversationListMenu,
  useConversationListActions,
} from "@/features/inbox/conversation-list-menu"
import { ConversationUnreadBadge } from "@/features/inbox/conversation-unread-badge"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import {
  useConversationTime,
  useMinuteTick,
} from "@/features/inbox/use-conversation-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { messagePreview } from "@/lib/message-preview"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

/** 会话列表。 */
export function InboxConversationList({
  conversations,
  onMenuChange,
  selectedId,
  onSelect,
}: {
  conversations: InboxConversation[]
  onMenuChange: (open: boolean) => void
  selectedId?: string
  onSelect: (conversationId: string) => void
}) {
  const { t } = useTranslation("inbox")
  const conversationName = useConversationName()
  const formatTime = useConversationTime()
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const actions = useConversationListActions()
  useMinuteTick()

  return (
      <>
        {conversations.map((conversation) => {
          const name = isAgentInboxConversation(conversation)
            ? `${conversation.agent.agentName} · ${conversation.agent.title}`
            : conversationName(conversation)
          const summary = isCustomerInboxConversation(conversation)
            ? conversation.customer
            : isAgentInboxConversation(conversation)
              ? conversation.agent
              : isDirectInboxConversation(conversation)
                ? conversation.direct
                : isGroupInboxConversation(conversation)
                  ? conversation.group
                  : null
          if (!summary) return null
          const agentRunLabel = agentRunStatusLabel(
            isAgentInboxConversation(conversation)
              ? conversation.agent.agentRunStatus
              : null,
            t,
          )
          const groupDissolved =
            isGroupInboxConversation(conversation) &&
            conversation.group.status ===
              ConversationStatus.ConversationStatusArchived
          const previewBody = groupDissolved
            ? t("groupDissolved")
            : conversation.lastMessageType === MessageType.MessageTypeAgentCancelled
              ? t("agentReplyStopped")
              : conversation.lastMessageType === MessageType.MessageTypeAgentError
                ? t("agentRunFailed")
                : messagePreview(
                    summary.preview ?? "",
                    summary.previewSenderIdentityType,
                  ).trim() ||
                  (isGroupInboxConversation(conversation) && summary.lastMessageAt
                    ? t("groupSystemUpdated")
                    : t("messagesEmpty"))
          // 客户会话的末条消息是内部备注时，摘要标明来源。
          const preview =
            isCustomerInboxConversation(conversation) &&
            conversation.customer.previewVisibility ===
              MessageVisibility.MessageVisibilityInternalOnly
              ? t("previewInternalNote", { preview: previewBody })
              : previewBody
          const formattedTime = formatTime(summary.lastMessageAt)
          const isInternal = isInternalInboxConversation(conversation)
          return (
            <ConversationListMenu
              key={conversation.id}
              conversation={conversation}
              actions={actions}
              onOpenChange={onMenuChange}
            >
                <button
                  type="button"
                  data-inbox-id={conversation.id}
                  aria-pressed={selectedId === conversation.id}
                  aria-label={name}
                  className={cn(
                    "flex h-[68px] w-full min-w-0 items-start gap-3 px-3 py-2.5 text-left transition-colors",
                    selectedId === conversation.id
                      ? "bg-accent text-accent-foreground"
                      : "hover:bg-muted",
                  )}
                  onClick={() => {
                    // 再次点击也按进入会话处理，等待在途的手动标记完成后清除。
                    if (selectedId === conversation.id && isInternal) {
                      void updateConversationUnreadMark(conversation.id, {
                        markedUnread: false,
                      })
                        .then(() => invalidate(resourceKeys.inbox()))
                        .catch((error: unknown) => {
                          console.warn("清除会话未读标记失败", {
                            conversationId: conversation.id,
                            error,
                          })
                          recoverSession(error, navigate)
                        })
                    }
                    onSelect(conversation.id)
                  }}
                >
                  <span className="relative shrink-0">
                    <ConversationAvatar conversation={conversation} />
                    <ConversationUnreadBadge conversation={conversation} />
                  </span>
                  <span className="min-w-0 flex-1 overflow-hidden">
                    <span className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="min-w-0 flex-1 truncate text-sm font-medium">
                          {name}
                        </span>
                        {agentRunLabel ? (
                          <span className="shrink-0 text-[10px] text-muted-foreground">
                            {agentRunLabel}
                          </span>
                        ) : null}
                      </span>
                      <span className="flex shrink-0 items-center gap-1.5">
                        {formattedTime ? (
                          <time
                            dateTime={summary.lastMessageAt ?? undefined}
                            className={cn(
                              "shrink-0 text-xs text-muted-foreground",
                              selectedId === conversation.id &&
                                "text-accent-foreground/75",
                            )}
                          >
                            {formattedTime}
                          </time>
                        ) : null}
                      </span>
                    </span>
                    <span className="mt-0.5 flex min-w-0 items-center gap-2">
                      <span
                        title={preview}
                        className={cn(
                          "min-w-0 flex-1 truncate text-xs text-muted-foreground",
                          selectedId === conversation.id &&
                            "text-accent-foreground/75",
                        )}
                      >
                        {preview}
                      </span>
                      {isInternal && conversation.muted ? (
                        <BellOffIcon
                          className="size-3.5 shrink-0 text-muted-foreground"
                          aria-label={t("conversationMuted")}
                        />
                      ) : null}
                    </span>
                  </span>
                </button>
            </ConversationListMenu>
          )
        })}
      </>
  )
}
