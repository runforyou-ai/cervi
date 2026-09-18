/** 会话列表项的阅读状态、静音与置顶操作及右键与长按菜单。 */
import type { ReactElement } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  isInternalInboxConversation,
  markConversationRead,
  updateConversationNotificationSettings,
  updateConversationPin,
  updateConversationUnreadMark,
  type ConversationPinCommand,
  type InboxConversation,
} from "@/api"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { resourceKeys } from "@/hooks/resource-keys"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const readStateFailure = {
  log: "更新会话阅读状态失败",
  message: "conversationReadStateError",
} as const

/** 串行保存列表中的会话个人设置，成功后重新读取统一收件箱；onPinSettled 在置顶写入成功后按目标分区优先的顺序重读列表。 */
export function useConversationListActions(onPinSettled?: (pinned: boolean) => Promise<void>) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const settingsSave = useImmediateSave()

  /** 执行一项个人设置保存，失败时按错误原因提示。 */
  async function save(
    conversation: InboxConversation,
    update: () => Promise<unknown>,
    failure: {
      log: string
      message:
        | "conversationReadStateError"
        | "conversationMuteError"
        | "conversationPinError"
      reload?: boolean
    },
  ) {
    const request = settingsSave.begin()
    if (request === null) return
    try {
      await update()
      await invalidate(resourceKeys.inbox())
    } catch (error) {
      if (!settingsSave.isCurrent(request)) return
      console.warn(failure.log, { conversationId: conversation.id, error })
      // 置顶写入失败后重读列表，恢复权威的置顶顺序与顺序版本。
      if (failure.reload) void invalidate(resourceKeys.inbox())
      if (!recoverSession(error, navigate)) {
        toast.error(isApiError(error) ? apiErrorMessage(error) : t(failure.message))
      }
    } finally {
      settingsSave.finish(request)
    }
  }

  return {
    saving: settingsSave.saving,
    // 有消息时把真实已读推进到列表最后一条消息并清除手动未读。
    markRead: (conversation: InboxConversation) =>
      save(
        conversation,
        () =>
          conversation.lastMessageId
            ? markConversationRead(conversation.id, {
                lastReadMessageId: conversation.lastMessageId,
                clearUnreadMark: true,
              })
            : updateConversationUnreadMark(conversation.id, {
                markedUnread: false,
              }),
        readStateFailure,
      ),
    markUnread: (conversation: InboxConversation) =>
      save(
        conversation,
        () =>
          updateConversationUnreadMark(conversation.id, { markedUnread: true }),
        readStateFailure,
      ),
    toggleMuted: (conversation: InboxConversation) =>
      save(
        conversation,
        () =>
          updateConversationNotificationSettings(conversation.id, {
            muted: !conversation.muted,
          }),
        { log: "更新会话静音设置失败", message: "conversationMuteError" },
      ),
    // 置顶、取消置顶与按邻居移动共用同一条写入，顺序版本冲突由服务端拒绝。
    updatePin: (conversation: InboxConversation, command: ConversationPinCommand) =>
      save(conversation, async () => {
        await updateConversationPin(conversation.id, command)
        await onPinSettled?.(command.pinned)
      }, {
        log: "更新会话置顶失败",
        message: "conversationPinError",
        reload: true,
      }),
  }
}

/** 为会话列表项提供阅读状态、静音与置顶菜单，右键或长按触发，没有可用操作时不打开；传入 pinOrderVersion 时提供置顶操作。 */
export function ConversationListMenu({
  conversation,
  actions,
  itemClassName,
  pinOrderVersion,
  onOpenChange,
  children,
}: {
  conversation: InboxConversation
  actions: ReturnType<typeof useConversationListActions>
  itemClassName?: string
  pinOrderVersion?: string
  onOpenChange: (open: boolean) => void
  children: ReactElement
}) {
  const { t } = useTranslation("inbox")
  const hasUnread = conversation.unreadCount > 0 || conversation.markedUnread
  const internal = isInternalInboxConversation(conversation)

  return (
    <ContextMenu onOpenChange={onOpenChange}>
      <ContextMenuTrigger asChild disabled={!internal && !hasUnread && pinOrderVersion === undefined}>
        {children}
      </ContextMenuTrigger>
      <ContextMenuContent>
        {hasUnread ? (
          <ContextMenuItem
            className={itemClassName}
            disabled={actions.saving}
            onSelect={() => void actions.markRead(conversation)}
          >
            {t("conversationMarkRead")}
          </ContextMenuItem>
        ) : null}
        {internal && !conversation.markedUnread ? (
          <ContextMenuItem
            className={itemClassName}
            disabled={actions.saving}
            onSelect={() => void actions.markUnread(conversation)}
          >
            {t("conversationMarkUnread")}
          </ContextMenuItem>
        ) : null}
        {internal ? (
          <ContextMenuItem
            className={itemClassName}
            disabled={actions.saving}
            onSelect={() => void actions.toggleMuted(conversation)}
          >
            {t(conversation.muted ? "conversationUnmute" : "conversationMute")}
          </ContextMenuItem>
        ) : null}
        {pinOrderVersion !== undefined ? (
          <ContextMenuItem
            className={itemClassName}
            disabled={actions.saving}
            onSelect={() =>
              void actions.updatePin(conversation, {
                pinned: !conversation.pinned,
                expectedPinOrderVersion: pinOrderVersion,
              })
            }
          >
            {t(conversation.pinned ? "conversationUnpin" : "conversationPin")}
          </ContextMenuItem>
        ) : null}
      </ContextMenuContent>
    </ContextMenu>
  )
}
