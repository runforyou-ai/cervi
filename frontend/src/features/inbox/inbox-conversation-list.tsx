/** 消息页中栏的会话列表项和右键操作。 */
import { BellOffIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ConversationStatus,
  MessageType,
  isAgentInboxConversation,
  isApiError,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  updateConversationNotificationSettings,
  updateConversationUnreadMark,
  type InboxConversation,
} from "@/api"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import {
  useConversationTime,
  useMinuteTick,
} from "@/features/inbox/use-conversation-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { messagePreview } from "@/lib/message-preview"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

/** 会话列表。 */
export function InboxConversationList({
  conversations,
  onMenuChange,
  selectedId,
  onSelect,
  onMarkRead,
}: {
  conversations: InboxConversation[]
  onMenuChange: (open: boolean) => void
  selectedId?: string
  onSelect: (conversationId: string) => void
  onMarkRead: (conversation: InboxConversation) => Promise<void>
}) {
  const { t } = useTranslation("inbox")
  const conversationName = useConversationName()
  const formatTime = useConversationTime()
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const settingsSave = useImmediateSave()
  useMinuteTick()

  /** 保存会话静音设置并重新读取统一收件箱。 */
  async function toggleConversationMuted(conversation: InboxConversation) {
    const request = settingsSave.begin()
    if (request === null) return
    try {
      await updateConversationNotificationSettings(conversation.id, {
        muted: !conversation.muted,
      })
      await invalidate(resourceKeys.inbox())
    } catch (error) {
      if (!settingsSave.isCurrent(request)) return
      console.warn("更新会话静音设置失败", {
        conversationId: conversation.id,
        error,
      })
      if (!recoverSession(error, navigate)) {
        toast.error(
          isApiError(error)
            ? apiErrorMessage(error)
            : t("conversationMuteError"),
        )
      }
    } finally {
      settingsSave.finish(request)
    }
  }

  /** 保存手动未读状态，按需把真实已读推进到列表最后一条消息。 */
  async function changeConversationReadState(
    conversation: InboxConversation,
    markedUnread: boolean,
    advanceRead = false,
  ) {
    const request = settingsSave.begin()
    if (request === null) return
    try {
      if (advanceRead && conversation.lastMessageId) {
        await onMarkRead(conversation)
      } else {
        await updateConversationUnreadMark(conversation.id, { markedUnread })
      }
      await invalidate(resourceKeys.inbox())
    } catch (error) {
      if (!settingsSave.isCurrent(request)) return
      console.warn("更新会话阅读状态失败", {
        conversationId: conversation.id,
        error,
      })
      if (!recoverSession(error, navigate)) {
        toast.error(
          isApiError(error)
            ? apiErrorMessage(error)
            : t("conversationReadStateError"),
        )
      }
    } finally {
      settingsSave.finish(request)
    }
  }

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
          const preview = groupDissolved
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
          const formattedTime = formatTime(summary.lastMessageAt)
          const hasUnread =
            conversation.unreadCount > 0 || conversation.markedUnread
          const isInternal =
            isAgentInboxConversation(conversation) ||
            isDirectInboxConversation(conversation) ||
            isGroupInboxConversation(conversation)
          return (
            <ContextMenu key={conversation.id} onOpenChange={onMenuChange}>
              <ContextMenuTrigger asChild>
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
                    {hasUnread ? (
                      <span
                        className={cn(
                          "absolute rounded-full ring-2 ring-background",
                          conversation.unreadCount > 0
                            ? "-top-1.5 -right-1.5 flex min-w-5 items-center justify-center gap-0.5 px-1 text-[10px] font-semibold leading-5"
                            : "-top-0.5 -right-0.5 size-2.5",
                          conversation.muted
                            ? "bg-muted text-muted-foreground"
                            : "bg-destructive text-destructive-foreground",
                        )}
                      >
                        {conversation.unreadCount > 0 ? (
                          <>
                            {conversation.mentionedUnreadCount > 0 ? "@" : null}
                            {conversation.unreadCount > 99
                              ? "99+"
                              : conversation.unreadCount}
                          </>
                        ) : (
                          <span className="sr-only">
                            {t("conversationMarkedUnread")}
                          </span>
                        )}
                      </span>
                    ) : null}
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
              </ContextMenuTrigger>
              {isInternal || hasUnread ? (
                <ContextMenuContent>
                  {hasUnread ? (
                    <ContextMenuItem
                      disabled={settingsSave.saving}
                      onSelect={() =>
                        void changeConversationReadState(
                          conversation,
                          false,
                          true,
                        )
                      }
                    >
                      {t("conversationMarkRead")}
                    </ContextMenuItem>
                  ) : null}
                  {isInternal && !conversation.markedUnread ? (
                    <ContextMenuItem
                      disabled={settingsSave.saving}
                      onSelect={() =>
                        void changeConversationReadState(conversation, true)
                      }
                    >
                      {t("conversationMarkUnread")}
                    </ContextMenuItem>
                  ) : null}
                  {isInternal ? (
                    <ContextMenuItem
                      disabled={settingsSave.saving}
                      onSelect={() =>
                        void toggleConversationMuted(conversation)
                      }
                    >
                      {t(
                        conversation.muted
                          ? "conversationUnmute"
                          : "conversationMute",
                      )}
                    </ContextMenuItem>
                  ) : null}
                </ContextMenuContent>
              ) : null}
            </ContextMenu>
          )
        })}
      </>
  )
}
