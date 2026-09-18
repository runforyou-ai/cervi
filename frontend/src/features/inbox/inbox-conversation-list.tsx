/** 消息页中栏的会话列表项、右键操作与置顶区排序。 */
import { useState } from "react"
import type { TFunction } from "i18next"
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core"
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { BellOffIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  ConversationPinPosition,
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

/** 会话列表项的摘要文案：群解散、运行结果与内部备注各有固定文案，其余取末条消息预览。 */
function conversationPreview(
  conversation: InboxConversation,
  summary: { preview?: string | null; previewSenderIdentityType?: Parameters<typeof messagePreview>[1]; lastMessageAt?: string | null },
  t: TFunction<"inbox">,
) {
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
  return isCustomerInboxConversation(conversation) &&
    conversation.customer.previewVisibility ===
      MessageVisibility.MessageVisibilityInternalOnly
    ? t("previewInternalNote", { preview: previewBody })
    : previewBody
}

type ConversationRowProps = {
  conversation: InboxConversation
  name: string
  selected: boolean
  actions: ReturnType<typeof useConversationListActions>
  pinOrderVersion: string
  onMenuChange: (open: boolean) => void
  onSelect: (conversationId: string) => void
  onOpenInWindow?: (conversation: InboxConversation, name: string) => void
  sortable?: ReturnType<typeof useSortable>
}

/** 会话列表项；置顶项使用浅色底，传入 sortable 时接入拖动与键盘排序。 */
function ConversationRow({
  conversation,
  name,
  selected,
  actions,
  pinOrderVersion,
  onMenuChange,
  onSelect,
  onOpenInWindow,
  sortable,
}: ConversationRowProps) {
  const { t } = useTranslation("inbox")
  const formatTime = useConversationTime()
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
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
  const preview = conversationPreview(conversation, summary, t)
  const formattedTime = formatTime(summary.lastMessageAt)
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
          style={
            sortable
              ? {
                  transform: CSS.Translate.toString(sortable.transform),
                  transition: sortable.transition,
                }
              : undefined
          }
          {...sortable?.attributes}
          {...sortable?.listeners}
          data-inbox-id={conversation.id}
          data-pinned={conversation.pinned || undefined}
          aria-pressed={selected}
          aria-label={name}
          className={cn(
            "flex h-[68px] w-full min-w-0 items-start gap-3 px-3 py-2.5 text-left transition-colors",
            selected
              ? "bg-accent text-accent-foreground"
              : conversation.pinned
                ? "bg-muted/60 hover:bg-muted"
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
                      selected &&
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
                  selected &&
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
}

/** 置顶区内可拖动与键盘排序的会话项，保存期间停用排序。 */
function SortableConversationRow(props: ConversationRowProps) {
  const { t } = useTranslation("inbox")
  const sortable = useSortable({
    id: props.conversation.id,
    disabled: props.actions.saving,
    attributes: { roleDescription: t("pinSortRole") },
  })
  return <ConversationRow {...props} sortable={sortable} />
}

/** 会话列表，置顶区在前且可在区内排序；传入 onOpenInWindow 时双击会话项在独立窗口打开该会话。 */
export function InboxConversationList({
  conversations,
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
  pinnedIds: string[]
  pinOrderVersion: string
  onMenuChange: (open: boolean) => void
  onDraggingChange: (active: boolean) => void
  onPinSettled: (pinned: boolean) => Promise<void>
  selectedId?: string
  onSelect: (conversationId: string) => void
  onOpenInWindow?: (conversation: InboxConversation, name: string) => void
}) {
  const { t } = useTranslation("inbox")
  const conversationName = useConversationName()
  const actions = useConversationListActions(onPinSettled)
  const [pendingOrder, setPendingOrder] = useState<string[] | null>(null)
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
      keyboardCodes: { start: ["Space"], cancel: ["Escape"], end: ["Space", "Enter"] },
    }),
  )
  useMinuteTick()

  const names = new Map(
    conversations.map((conversation) => [
      conversation.id,
      isAgentInboxConversation(conversation)
        ? `${conversation.agent.agentName} · ${conversation.agent.title}`
        : conversationName(conversation),
    ]),
  )
  const rows = new Map(conversations.map((conversation) => [conversation.id, conversation]))
  // 拖动放下后先按临时顺序展示，置顶区权威顺序重读完成或保存失败后撤销。
  const pinnedOrder =
    pendingOrder?.length === pinnedIds.length && pendingOrder.every((id) => pinnedIds.includes(id))
      ? pendingOrder
      : pinnedIds
  const row = (conversation: InboxConversation) => ({
    conversation,
    name: names.get(conversation.id) ?? "",
    selected: selectedId === conversation.id,
    actions,
    pinOrderVersion,
    onMenuChange,
    onSelect,
    onOpenInWindow,
  })
  const announce = (key: "pinSortPicked" | "pinSortMoved" | "pinSortDropped" | "pinSortCancelled", id: string | number, overId?: string | number) =>
    t(key, { name: names.get(String(id)) ?? "", position: pinnedOrder.indexOf(String(overId ?? id)) + 1, total: pinnedOrder.length })

  /** 把放下位置转成相对可见邻居的位置命令并保存。 */
  function dropPinned({ active, over }: DragEndEvent) {
    onDraggingChange(false)
    const from = pinnedOrder.indexOf(String(active.id))
    const to = over ? pinnedOrder.indexOf(String(over.id)) : -1
    const conversation = rows.get(String(active.id))
    if (!conversation || from < 0 || to < 0 || from === to) return
    setPendingOrder(arrayMove(pinnedOrder, from, to))
    void actions
      .updatePin(conversation, {
        pinned: true,
        position:
          from > to
            ? ConversationPinPosition.ConversationPinPositionBefore
            : ConversationPinPosition.ConversationPinPositionAfter,
        neighborId: pinnedOrder[to],
        expectedPinOrderVersion: pinOrderVersion,
      })
      .finally(() => setPendingOrder(null))
  }

  return (
    <>
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        modifiers={[({ transform }) => ({ ...transform, x: 0 })]}
        accessibility={{
          screenReaderInstructions: { draggable: t("pinSortInstructions") },
          announcements: {
            onDragStart: ({ active }) => announce("pinSortPicked", active.id),
            onDragOver: ({ active, over }) => over ? announce("pinSortMoved", active.id, over.id) : undefined,
            onDragEnd: ({ active, over }) => announce("pinSortDropped", active.id, over?.id),
            onDragCancel: ({ active }) => announce("pinSortCancelled", active.id),
          },
        }}
        onDragStart={() => onDraggingChange(true)}
        onDragEnd={dropPinned}
        onDragCancel={() => onDraggingChange(false)}
      >
        <SortableContext items={pinnedOrder} strategy={verticalListSortingStrategy}>
          {pinnedOrder.flatMap((id) => rows.has(id) ? [<SortableConversationRow key={id} {...row(rows.get(id)!)} />] : [])}
        </SortableContext>
      </DndContext>
      {conversations.flatMap((conversation) => pinnedIds.includes(conversation.id) ? [] : [<ConversationRow key={conversation.id} {...row(conversation)} />])}
    </>
  )
}
