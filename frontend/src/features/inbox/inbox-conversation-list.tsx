/** 消息页中栏的会话列表项、右键操作与置顶区排序。 */
import { useNavigate } from "react-router"

import {
  isCustomerInboxConversation,
  isInternalInboxConversation,
  updateConversationUnreadMark,
  type InboxConversation,
} from "@/api"
import {
  ConversationListMenu,
  useConversationListActions,
} from "@/features/inbox/conversation-list-menu"
import { inboxConversationSummary } from "@/features/inbox/conversation-preview"
import { ConversationRowContent } from "@/features/inbox/conversation-row-content"
import {
  PinnedConversationList,
  pinSortableStyle,
  usePinSortable,
  type PinSortable,
} from "@/features/inbox/pinned-sort"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import { useMinuteTick } from "@/features/inbox/use-conversation-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

type ConversationRowProps = {
  conversation: InboxConversation
  name: string
  showQueueTeam: boolean
  showAssignee: boolean
  selected: boolean
  actions: ReturnType<typeof useConversationListActions>
  pinOrderVersion: string
  onMenuChange: (open: boolean) => void
  onSelect: (conversationId: string) => void
  onOpenInWindow?: (conversation: InboxConversation, name: string) => void
  sortable?: PinSortable
}

/** 会话列表项；置顶项使用与悬停一致的底色，传入 sortable 时接入拖动与键盘排序。 */
function ConversationRow({
  conversation,
  name,
  showQueueTeam,
  showAssignee,
  selected,
  actions,
  pinOrderVersion,
  onMenuChange,
  onSelect,
  onOpenInWindow,
  sortable,
}: ConversationRowProps) {
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  if (!inboxConversationSummary(conversation)) return null
  // 按全部队列查看待分配会话时标明所属团队队列，公共队列不加标签。
  const queueTeamName =
    showQueueTeam && isCustomerInboxConversation(conversation)
      ? conversation.customer.teamName
      : null
  const isInternal = isInternalInboxConversation(conversation)
  return (
    <ConversationListMenu
      conversation={conversation}
      actions={actions}
      pinOrderVersion={pinOrderVersion}
      onOpenChange={onMenuChange}
    >
        <button
          type="button"
          ref={sortable?.setNodeRef}
          style={pinSortableStyle(sortable)}
          {...sortable?.attributes}
          {...sortable?.listeners}
          data-inbox-id={conversation.id}
          data-pinned={conversation.pinned || undefined}
          aria-pressed={selected}
          aria-label={name}
          className={cn(
            "flex h-[56px] w-full min-w-0 items-start gap-2.5 px-2.5 py-2 text-left transition-colors",
            selected
              ? "bg-accent text-accent-foreground"
              : conversation.pinned
                ? "bg-muted"
                : "hover:bg-muted",
            sortable?.isDragging && "relative z-10 shadow-md",
          )}
          onClick={() => {
            // 再次点击也按进入会话处理，等待在途的手动标记完成后清除。
            if (selected && isInternal) {
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
          onDoubleClick={
            onOpenInWindow
              ? () => onOpenInWindow(conversation, name)
              : undefined
          }
        >
          <ConversationRowContent
            conversation={conversation}
            name={name}
            density="compact"
            selected={selected}
            showAssignee={showAssignee}
            queueTeamName={queueTeamName}
          />
        </button>
    </ConversationListMenu>
  )
}

/** 置顶区内可拖动与键盘排序的会话项，保存期间停用排序。 */
function SortableConversationRow(props: ConversationRowProps) {
  const sortable = usePinSortable(props.conversation.id, props.actions.saving)
  return <ConversationRow {...props} sortable={sortable} />
}

/** 会话列表，置顶区在前且可在区内排序；showQueueTeam 为真时客户会话名称后显示所属团队队列，showAssignee 为真时客户会话摘要行末显示负责人小头像，传入 onOpenInWindow 时双击会话项在独立窗口打开该会话。 */
export function InboxConversationList({
  conversations,
  showQueueTeam,
  showAssignee,
  pinnedIds,
  pinOrderVersion,
  onMenuChange,
  onDraggingChange,
  onPinSettled,
  selectedId,
  onSelect,
  onOpenInWindow,
}: {
  conversations: InboxConversation[]
  showQueueTeam: boolean
  showAssignee: boolean
  pinnedIds: string[]
  pinOrderVersion: string
  onMenuChange: (open: boolean) => void
  onDraggingChange: (active: boolean) => void
  onPinSettled: (pinned: boolean) => Promise<void>
  selectedId?: string
  onSelect: (conversationId: string) => void
  onOpenInWindow?: (conversation: InboxConversation, name: string) => void
}) {
  const conversationName = useConversationName()
  const actions = useConversationListActions(onPinSettled)
  useMinuteTick()

  const names = new Map(
    conversations.map((conversation) => [
      conversation.id,
      conversationName(conversation),
    ]),
  )
  const row = (conversation: InboxConversation) => ({
    conversation,
    name: names.get(conversation.id) ?? "",
    showQueueTeam,
    showAssignee,
    selected: selectedId === conversation.id,
    actions,
    pinOrderVersion,
    onMenuChange,
    onSelect,
    onOpenInWindow,
  })
  return (
    <PinnedConversationList
      conversations={conversations}
      pinnedIds={pinnedIds}
      names={names}
      pinOrderVersion={pinOrderVersion}
      actions={actions}
      onDraggingChange={onDraggingChange}
      renderPinned={(conversation) => (
        <SortableConversationRow key={conversation.id} {...row(conversation)} />
      )}
      renderRow={(conversation) => (
        <ConversationRow key={conversation.id} {...row(conversation)} />
      )}
    />
  )
}
