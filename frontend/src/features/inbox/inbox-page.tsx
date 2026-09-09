/** 消息页中栏（范围纵栏 + 会话列表）和会话主区。 */
import {
  useCallback,
  useEffect,
  useEffectEvent,
  useRef,
  useState,
} from "react"
import { useQueryClient } from "@tanstack/react-query"
import { clearConversationResources } from "./conversation-resources"
import {
  BellOffIcon,
  CheckIcon,
  ChevronDownIcon,
  HeadsetIcon,
  MessagesSquareIcon,
  PanelLeftIcon,
  PlusIcon,
  SearchIcon,
  UsersRoundIcon,
} from "lucide-react"
import { messagePreview } from "@/lib/message-preview"
import { useConversationSummary, readConversationSummary } from "./use-conversation-summary"
import { useAttachmentQueue } from "./attachment-queue-context"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ChannelType,
  ConversationStatus,
  ConversationType,
  MessageType,
  CustomerInboxView,
  InboxScope,
  OrganizationIdentityType,
  ServiceSessionStatus,
  isApiError,
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  getGroupConversation,
  listCustomerServiceAssignees,
  markConversationRead,
  sendFirstAgentTextMessage,
  sendFirstDirectTextMessage,
  updateConversationNotificationSettings,
  updateConversationUnreadMark,
  type ConversationMessageReference,
  type CustomerInboxConversationData,
  type AgentInboxConversationData,
  type DirectInboxConversationData,
  type GroupInboxConversationData,
  type InboxAssignee,
  type InboxConversation,
  type MemberOption,
} from "@/api"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import { PageSplit } from "@/components/page-split"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { usePortalContainer } from "@/components/ui/portal-container"
import { InboxListPanel } from "./inbox-list-panel"
import type { InboxList } from "./use-inbox-list"
import type { useInboxListViewport } from "./use-inbox-list-viewport"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useWorkspace } from "@/contexts/workspace-context"
import { sessionStatusLabel } from "@/features/inbox/session-status-label"
import { useConversationTime, useMinuteTick } from "@/features/inbox/use-conversation-time"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import {
  ConversationComposer,
} from "@/features/inbox/conversation-composer"
import { ConversationContextPane } from "@/features/inbox/conversation-context-pane"
import { ConversationHeader } from "@/features/inbox/conversation-header"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import { CreateGroupConversationDialog } from "@/features/inbox/create-group-conversation-dialog"
import { DirectConversationDraftHeader } from "@/features/inbox/direct-conversation-draft-header"
import { InboxConversationTarget } from "@/features/inbox/inbox-conversation-target"
import { ConversationTargetPickerDialog } from "@/features/inbox/conversation-target-picker-dialog"
import {
  useOutgoingConversationMessages,
  type OutgoingConversationDraft,
} from "@/features/inbox/use-outgoing-conversation-messages"
import {
  useIsNarrowViewport,
  useIsWideViewport,
} from "@/hooks/use-narrow-viewport"
import { resourceKeys } from "@/hooks/resource-keys"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

type ChatDraft =
  | { kind: "direct-draft"; member: MemberOption }
  | { kind: "agent-draft"; member: MemberOption; conversationId: string }

type ConversationSelection =
  ChatDraft | { kind: "conversation"; conversation: InboxConversation }

type InternalInboxConversationData =
  | AgentInboxConversationData
  | DirectInboxConversationData
  | GroupInboxConversationData

const scopes = [
  {
    id: InboxScope.InboxScopeAll,
    labelKey: "scopeAll",
    icon: MessagesSquareIcon,
  },
  {
    id: InboxScope.InboxScopeCustomer,
    labelKey: "scopeCustomer",
    icon: HeadsetIcon,
  },
  {
    id: InboxScope.InboxScopeInternal,
    labelKey: "scopeInternal",
    icon: UsersRoundIcon,
  },
] as const

/** 会话在列表和主区中的显示名。 */
function useConversationName() {
  const { t } = useTranslation("inbox")
  return useCallback(
    (conversation: InboxConversation) => {
      if (isAgentInboxConversation(conversation))
        return conversation.agent.title
      if (isDirectInboxConversation(conversation)) {
        return conversation.direct.peerName.trim() || t("unknownSender")
      }
      if (isCustomerInboxConversation(conversation)) {
        return (
          conversation.customer.contactName?.trim() || t("anonymousVisitor")
        )
      }
      if (isGroupInboxConversation(conversation)) {
        return conversation.group.title.trim() || t("unknownSender")
      }
      return t("unknownSender")
    },
    [t],
  )
}

/** 顶部操作行：收纳范围栏、搜索占位和发起会话菜单。 */
function InboxPaneTop({
  railCollapsed,
  onRailToggle,
  onCreateGroup,
  onCreateAgent,
}: {
  railCollapsed: boolean
  onRailToggle: () => void
  onCreateGroup: () => void
  onCreateAgent: () => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <div
      data-slot="inbox-pane-header"
      className="flex h-14 shrink-0 items-center gap-2 border-b border-border/60 px-3"
    >
      <Button
        variant="ghost"
        size="icon"
        className="shrink-0 text-muted-foreground"
        aria-pressed={railCollapsed}
        aria-label={
          railCollapsed ? t("scopeRailExpand") : t("scopeRailCollapse")
        }
        title={railCollapsed ? t("scopeRailExpand") : t("scopeRailCollapse")}
        onClick={onRailToggle}
      >
        <PanelLeftIcon className="size-5" />
      </Button>
      <div className="relative min-w-0 flex-1">
        <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          disabled
          aria-label={t("searchLabel")}
          className="h-9 w-full rounded-md border border-transparent bg-muted px-8 text-sm text-foreground opacity-50"
        />
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0 bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground"
            aria-label={t("newConversation")}
            title={t("newConversation")}
          >
            <PlusIcon />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="min-w-52">
          <DropdownMenuItem onSelect={onCreateAgent}>
            {t("newAgentConversation")}
          </DropdownMenuItem>
          <DropdownMenuItem className="gap-2" onSelect={onCreateGroup}>
            <span className="min-w-0 flex-1 truncate">
              {t("newGroupConversation")}
            </span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}

/** 中栏左缘的范围纵栏。 */
function InboxScopeRail({
  scope,
  attentionUnreadCount,
  onScopeChange,
}: {
  scope: InboxScope
  attentionUnreadCount: number
  onScopeChange: (scope: InboxScope) => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <nav
      aria-label={t("scopeRailLabel")}
      className="flex w-20 shrink-0 flex-col gap-0.5 overflow-y-auto border-r border-border/70 bg-muted/30 px-1.5 py-1.5"
    >
      {scopes.map((item) => (
        <button
          key={item.id}
          type="button"
          aria-pressed={scope === item.id}
          className={cn(
            "flex flex-col items-center gap-0.5 rounded-lg px-0 pt-1.5 pb-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
            scope === item.id &&
              "bg-accent font-medium text-accent-foreground hover:bg-accent hover:text-accent-foreground",
          )}
          onClick={() => onScopeChange(item.id)}
        >
          <span className="relative">
            <item.icon className="size-5" />
            {item.id === InboxScope.InboxScopeInternal &&
            attentionUnreadCount > 0 ? (
              <span
                className="absolute -top-0.5 -right-1 size-2 rounded-full bg-destructive"
                role="status"
                aria-label={t("internalAttentionUnread", {
                  count: attentionUnreadCount,
                })}
              />
            ) : null}
          </span>
          <span className="w-full truncate px-px text-center text-xs leading-tight">
            {t(item.labelKey)}
          </span>
        </button>
      ))}
    </nav>
  )
}

/** 客户范围的四个服务视图；同事视图在下拉中继续选择具体客服。 */
function InboxCustomerQueueFilter({
  view,
  assigneeIdentityId,
  assignees,
  currentIdentityId,
  onChange,
}: {
  view: CustomerInboxView
  assigneeIdentityId: string
  assignees: InboxAssignee[]
  currentIdentityId: string
  onChange: (view: CustomerInboxView, assigneeIdentityId?: string) => void
}) {
  const { t } = useTranslation("inbox")
  const coworkers = assignees.filter(
    (assignee) => assignee.identityId !== currentIdentityId,
  )
  const selectedCoworker = coworkers.find(
    (assignee) => assignee.identityId === assigneeIdentityId,
  )
  const segments = [
    {
      id: CustomerInboxView.CustomerInboxViewQueue,
      label: t("queueFilterQueue"),
    },
    {
      id: CustomerInboxView.CustomerInboxViewMine,
      label: t("queueFilterMine"),
    },
  ] as const

  function tabClass(active: boolean) {
    return cn(
      "relative h-9 min-w-0 flex-1 rounded-md px-1 text-center text-sm font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring",
      active
        ? "text-foreground"
        : "text-muted-foreground hover:text-foreground",
    )
  }

  function activeIndicator(active: boolean) {
    return active ? (
      <span
        aria-hidden="true"
        className="absolute right-1 -bottom-px left-1 h-0.5 rounded bg-primary"
      />
    ) : null
  }

  return (
    <div
      role="tablist"
      aria-label={t("queueFilterLabel")}
      className="flex shrink-0 items-stretch border-b border-border/60 px-2 pt-2"
    >
      {segments.map((segment) => {
        const active = view === segment.id
        return (
          <button
            key={segment.id}
            type="button"
            role="tab"
            aria-selected={active}
            className={tabClass(active)}
            onClick={() => onChange(segment.id)}
          >
            <span className="block truncate">{segment.label}</span>
            {activeIndicator(active)}
          </button>
        )
      })}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            role="tab"
            aria-selected={
              view === CustomerInboxView.CustomerInboxViewCoworkers
            }
            title={selectedCoworker?.displayName ?? t("queueFilterColleague")}
            className={tabClass(
              view === CustomerInboxView.CustomerInboxViewCoworkers,
            )}
          >
            <span className="flex min-w-0 items-center justify-center gap-0.5">
              <span className="truncate">{t("queueFilterColleague")}</span>
              <ChevronDownIcon className="size-3.5 opacity-70" />
            </span>
            {activeIndicator(
              view === CustomerInboxView.CustomerInboxViewCoworkers,
            )}
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="center" className="min-w-48">
          <DropdownMenuItem
            className={cn(
              view === CustomerInboxView.CustomerInboxViewCoworkers &&
                !assigneeIdentityId &&
                "bg-muted text-foreground",
            )}
            onSelect={() =>
              onChange(CustomerInboxView.CustomerInboxViewCoworkers, "")
            }
          >
            <span className="min-w-0 flex-1 truncate">
              {t("queueFilterAllCoworkers")}
            </span>
            {view === CustomerInboxView.CustomerInboxViewCoworkers &&
            !assigneeIdentityId ? (
              <CheckIcon className="size-4" />
            ) : null}
          </DropdownMenuItem>
          {coworkers.map((assignee) => (
            <DropdownMenuItem
              key={assignee.identityId}
              className={cn(
                assigneeIdentityId === assignee.identityId &&
                  "bg-muted text-foreground",
              )}
              onSelect={() =>
                onChange(
                  CustomerInboxView.CustomerInboxViewCoworkers,
                  assignee.identityId,
                )
              }
            >
              <span className="min-w-0 flex-1 truncate">
                {assignee.displayName}
              </span>
              {assignee.type ===
              OrganizationIdentityType.OrganizationIdentityTypeAgent ? (
                <span className="text-xs text-muted-foreground">
                  {t("queueFilterAiEmployee")}
                </span>
              ) : null}
              {assigneeIdentityId === assignee.identityId ? (
                <CheckIcon className="size-4" />
              ) : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <button
        type="button"
        role="tab"
        aria-selected={view === CustomerInboxView.CustomerInboxViewClosed}
        className={tabClass(view === CustomerInboxView.CustomerInboxViewClosed)}
        onClick={() => onChange(CustomerInboxView.CustomerInboxViewClosed)}
      >
        <span className="block truncate">{t("queueFilterClosed")}</span>
        {activeIndicator(view === CustomerInboxView.CustomerInboxViewClosed)}
      </button>
    </div>
  )
}

/** 会话列表。 */
function InboxConversationList({
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

/** 组合当前 Conversation 工作区和独立联系人上下文栏。 */
function ConversationMain({
  selection,
  onSessionChanged,
  onConversationChanged,
  onGroupLeft,
  onChatStarted,
  narrowViewport = false,
}: {
  selection: ConversationSelection
  onSessionChanged: (conversation: CustomerInboxConversationData) => void
  onConversationChanged: (
    conversation:
      | CustomerInboxConversationData
      | AgentInboxConversationData
      | DirectInboxConversationData
      | GroupInboxConversationData,
  ) => void
  onGroupLeft: (conversationID: string) => void
  onChatStarted: (
    conversation: DirectInboxConversationData | AgentInboxConversationData,
  ) => void
  narrowViewport?: boolean
}) {
  const conversation =
    selection.kind === "conversation" ? selection.conversation : null
  const directTarget =
    selection.kind !== "conversation" ? selection.member : null
  const { t } = useTranslation("inbox")
  const { identity } = useWorkspace()
  const isWideViewport = useIsWideViewport()
  const conversationName = useConversationName()
  const [contextCollapsed, setContextCollapsed] = useState(
    () => !isWideViewport,
  )

  useEffect(() => {
    // 跨过响应式断点时恢复当前宽度对应的默认状态。
    setContextCollapsed(!isWideViewport)
  }, [isWideViewport])

  const sourceGroupConversation =
    conversation && isGroupInboxConversation(conversation) ? conversation : null
  const groupPollingActive = useMemberChatPollingActive()
  const groupResource = useResource(
    resourceKeys.groupConversation(sourceGroupConversation?.id ?? ""),
    () => getGroupConversation(sourceGroupConversation?.id ?? ""),
    {
      enabled: Boolean(sourceGroupConversation),
      staleTime: 0,
      refetchInterval: groupPollingActive ? memberChatPollingInterval : false,
    },
  )
  const group = groupResource.data
  const displayedConversation =
    sourceGroupConversation && group
      ? {
          ...sourceGroupConversation,
          group: {
            ...sourceGroupConversation.group,
            title: group.title,
            imageUrl: group.imageUrl,
            memberCount: group.participants.length,
            status: group.status,
          },
        }
      : conversation
  const contactName = displayedConversation
    ? conversationName(displayedConversation)
    : (directTarget?.displayName ?? "")
  const customerConversation =
    displayedConversation && isCustomerInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const directConversation =
    displayedConversation && isDirectInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const groupConversation =
    displayedConversation && isGroupInboxConversation(displayedConversation)
      ? displayedConversation
      : null
  const sessionStatus = customerConversation
    ? sessionStatusLabel(customerConversation.customer.serviceSessionStatus, t)
    : ""
  const replyDisabledReason = customerConversation
    ? customerConversation.customer.serviceSessionStatus ===
      ServiceSessionStatus.ServiceSessionStatusClosed
      ? t("replyClosedUnavailable")
      : customerConversation.customer.assignee &&
          customerConversation.customer.assignee.identityId !==
            identity.user.identityId
        ? t("replyAssignedUnavailable", {
            name: customerConversation.customer.assignee.displayName,
          })
        : null
    : groupConversation?.group.status ===
        ConversationStatus.ConversationStatusArchived
      ? t("groupDissolvedUnavailable")
      : null
  const validConversation =
    customerConversation ??
    directConversation ??
    groupConversation ??
    (displayedConversation && isAgentInboxConversation(displayedConversation)
      ? displayedConversation
      : null)
  if (!validConversation && !directTarget) return null
  // AI 草稿与正式会话共用稳定编号，真人单聊按固定身份保持组件。
  const threadKey =
    selection.kind === "agent-draft"
      ? selection.conversationId
      : (directTarget?.id ??
        directConversation?.direct.peerIdentityId ??
        validConversation?.id)

  return (
    <div className="flex h-full min-h-0 bg-background">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        {validConversation ? (
          <ConversationHeader
            conversation={validConversation}
            contactName={contactName}
            sessionStatus={sessionStatus}
            currentIdentityId={identity.user.identityId}
            onSessionChanged={() => {
              if (customerConversation) onSessionChanged(customerConversation)
            }}
            narrowViewport={narrowViewport}
          />
        ) : directTarget ? (
          <DirectConversationDraftHeader member={directTarget} />
        ) : null}
        <ConversationThread
          key={threadKey}
          conversation={validConversation}
          directTarget={directTarget}
          agentDraftID={
            selection.kind === "agent-draft" ? selection.conversationId : ""
          }
          replyDisabledReason={replyDisabledReason}
          onConversationChanged={() => {
            if (validConversation) onConversationChanged(validConversation)
          }}
          onChatStarted={onChatStarted}
        />
      </div>
      <ConversationContextPane
        conversation={validConversation}
        directTarget={directTarget}
        displayName={contactName}
        currentIdentityID={identity.user.identityId}
        onGroupLeft={() => {
          if (validConversation) onGroupLeft(validConversation.id)
        }}
        visible={!contextCollapsed}
        onToggle={() => setContextCollapsed((collapsed) => !collapsed)}
      />
    </div>
  )
}

/** 协调当前会话时间线和回复区的即时消息。 */
function ConversationThread({
  agentDraftID,
  conversation,
  directTarget,
  replyDisabledReason,
  onConversationChanged,
  onChatStarted,
}: {
  conversation:
    | CustomerInboxConversationData
    | AgentInboxConversationData
    | DirectInboxConversationData
    | GroupInboxConversationData
    | null
  directTarget: MemberOption | null
  agentDraftID: string
  replyDisabledReason: string | null
  onConversationChanged: () => void
  onChatStarted: (
    conversation: DirectInboxConversationData | AgentInboxConversationData,
  ) => void
}) {
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const pageActive = usePortalContainer()?.active ?? true
  const { identity } = useWorkspace()
  const outgoing = useOutgoingConversationMessages()
  const { queue: attachmentQueue, jobs: attachmentJobs } = useAttachmentQueue()
  const invalidate = useResourceInvalidator()
  const aliveRef = useRef(true)
  const conversationID = conversation?.id ?? ""
  const conversationType =
    conversation?.type ??
    (agentDraftID
      ? ConversationType.ConversationTypeAgent
      : ConversationType.ConversationTypeDirect)

  const handleUnreadMarkClearError = useEffectEvent(
    (id: string, error: unknown) => {
      console.warn("清除会话未读标记失败", { conversationId: id, error })
      recoverSession(error, navigate)
    },
  )

  useEffect(() => {
    if (
      !conversationID ||
      !pageActive ||
      conversationType === ConversationType.ConversationTypeCustomer
    )
      return
    let current = true
    // 每次进入都清除服务端标记，避免旧缓存掩盖另一端新设的标记。
    void updateConversationUnreadMark(conversationID, { markedUnread: false })
      .then(() => invalidate(resourceKeys.inbox()))
      .catch((error: unknown) => {
        if (!current) return
        handleUnreadMarkClearError(conversationID, error)
      })
    return () => {
      current = false
    }
  }, [conversationID, conversationType, pageActive, invalidate])

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
    }
  }, [])
  const [replyTo, setReplyTo] = useState<ConversationMessageReference | null>(
    null,
  )
  const [retryDraft, setRetryDraft] =
    useState<OutgoingConversationDraft | null>(null)
  const messageSending = outgoing.messages.some(
    (message) => message.status === "sending",
  )
  const replySupported =
    !conversation ||
    isAgentInboxConversation(conversation) ||
    isDirectInboxConversation(conversation) ||
    isGroupInboxConversation(conversation) ||
    (conversation.customer.channelType === ChannelType.ChannelTypeWebsite ||
      conversation.customer.channelType === ChannelType.ChannelTypeTelegram)
  const telegramConversation = Boolean(
    conversation &&
    isCustomerInboxConversation(conversation) &&
    conversation.customer.channelType === ChannelType.ChannelTypeTelegram,
  )
  const groupConversation =
    conversation && isGroupInboxConversation(conversation) ? conversation : null
  const groupResource = useResource(
    resourceKeys.groupConversation(groupConversation?.id ?? ""),
    () => getGroupConversation(groupConversation?.id ?? ""),
    { enabled: Boolean(groupConversation) },
  )

  /** 保存当前已看到的最新消息并刷新收件箱未读摘要。 */
  const markRead = useCallback(
    (messageID: string) => {
      if (!conversation) return
      void markConversationRead(conversation.id, {
        lastReadMessageId: messageID,
        clearUnreadMark: false,
      })
        .then(() => {
          void invalidate(resourceKeys.inbox())
          void invalidate(resourceKeys.conversationSummary(conversation.id))
        })
        .catch((error: unknown) =>
          console.warn("标记会话已读失败", {
            conversationId: conversation.id,
            error,
          }),
        )
    },
    [conversation, invalidate],
  )

  return (
    <>
      <ConversationTimeline
        prepareSendRef={prepareSendRef}
        customerDeliveries={telegramConversation}
        conversationID={conversationID}
        conversationType={conversationType}
        currentUser={identity.user}
        outgoingMessages={[...outgoing.messages, ...attachmentJobs.filter(job => job.stage !== "cancelled" && (conversationID ? job.conversationID === conversationID : directTarget && job.targetIdentityID === directTarget.id)).map(job => job.message)]}
        onRetryFailedMessage={(draft) => {
          if (attachmentJobs.some(job => job.id === draft.clientMessageID)) attachmentQueue?.retry(draft.clientMessageID)
          else setRetryDraft(draft)
        }}
        retryFailedMessageDisabled={
          messageSending || !replySupported || Boolean(replyDisabledReason)
        }
        groupParticipants={groupResource.data?.participants}
        onReplyMessage={
          conversation && replySupported && !replyDisabledReason
            ? setReplyTo
            : undefined
        }
        onReadMessage={conversation ? markRead : undefined}
        readThroughMessageID={conversation?.lastReadMessageId}
        enabled={Boolean(conversation)}
      />
      <ConversationComposer
        disabledReason={!replySupported ? t("channelReplyUnsupported") : replyDisabledReason}
        onBeforeSend={() =>
          prepareSendRef.current?.() ?? Promise.resolve(true)
        }
        conversationID={conversationID}
        conversationType={conversationType}
        submitOnEnter
        refocusAfterSubmit
        retryFailedMessage
        retryDraft={retryDraft}
        replyTo={replyTo}
        groupParticipants={groupResource.data?.participants}
        currentIdentityID={identity.user.identityId}
        onRetryDraftHandled={() => setRetryDraft(null)}
        onReplyToChange={setReplyTo}
        onSending={outgoing.start}
        onSent={outgoing.succeed}
        onFailed={(clientMessageID) => {
          outgoing.fail(clientMessageID)
          if (conversationID) void invalidate(resourceKeys.conversationSummary(conversationID))
          // 发送被拒绝后同步群资料，及时关闭解散群的发送区。
          if (groupConversation)
            void invalidate(resourceKeys.groupConversation(groupConversation.id))
        }}
        onSucceeded={onConversationChanged}
        attachmentTargetIdentityID={directTarget && !agentDraftID ? directTarget.id : undefined}
        onAttachmentConversationCreated={(created) => {
          if (directTarget) void invalidate(resourceKeys.directConversation(directTarget.id))
          void invalidate(resourceKeys.conversationMessages(created.id))
          if (aliveRef.current && isDirectInboxConversation(created)) onChatStarted(created)
        }}
        sendIndividualMessage={
          directTarget
            ? async (input) => {
                const result = agentDraftID
                  ? await sendFirstAgentTextMessage({
                      conversationId: agentDraftID,
                      agentIdentityId: directTarget.id,
                      clientMessageId: input.clientMessageId,
                      body: input.body,
                    })
                  : await sendFirstDirectTextMessage({
                      targetIdentityId: directTarget.id,
                      ...input,
                    })
                if (!agentDraftID)
                  void invalidate(
                    resourceKeys.directConversation(directTarget.id),
                  )
                void invalidate(
                  resourceKeys.conversationMessages(result.conversation.id),
                )
                // 离开原线程后只刷新列表，不改变当前选择。
                if (aliveRef.current) {
                  onChatStarted(result.conversation)
                } else {
                  void invalidate(resourceKeys.inbox())
                }
                return result.message
              }
            : undefined
        }
      />
    </>
  )
}

/** 消息页中栏和当前会话。 */
export function InboxPage({
  list,
  listViewport,
  scope,
  customerView,
  assigneeIdentityId,
  selectedConversationId,
  targetIdentityId,
  onSelectedConversationChange,
  onQueryChange,
}: {
  list: InboxList
  listViewport: ReturnType<typeof useInboxListViewport>
  scope: InboxScope
  customerView: CustomerInboxView
  assigneeIdentityId: string
  selectedConversationId: string
  targetIdentityId: string
  onSelectedConversationChange: (
    conversationId: string,
    replace?: boolean,
  ) => void
  onQueryChange: (changes: {
    scope?: InboxScope
    customerView?: CustomerInboxView
    assigneeIdentityId?: string
    conversationId?: string
    replace?: boolean
  }) => void
}) {
  const { conversations, attentionUnreadCount } = list
  // 空窗口或尚未完成首次读取时，不把整个筛选解释为没有会话。
  const hasConversations = conversations.length > 0 || list.hasBefore || list.hasAfter || list.revision === 0
  const { t } = useTranslation(["inbox", "common"])
  const { identity } = useWorkspace()
  const isNarrowViewport = useIsNarrowViewport()
  const invalidate = useResourceInvalidator()
  const queryClient = useQueryClient()
  const { queue } = useAttachmentQueue()
  const [railCollapsed, setRailCollapsed] = useState(false)
  const [chatDraft, setChatDraft] = useState<ChatDraft | null>(null)
  const [isNarrowDetailOpen, setIsNarrowDetailOpen] = useState(false)
  const [agentDialogOpen, setAgentDialogOpen] = useState(false)
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const navigationGeneration = useRef(0)
  const summary = useConversationSummary(targetIdentityId ? "" : selectedConversationId)
  const selectedConversation = summary.data ?? undefined
  useEffect(() => {
    navigationGeneration.current++
    return () => { navigationGeneration.current++ }
  }, [scope, customerView, assigneeIdentityId, selectedConversationId, targetIdentityId, chatDraft])
  useEffect(() => {
    if (isNarrowViewport && selectedConversationId) setIsNarrowDetailOpen(true)
  }, [isNarrowViewport, selectedConversationId])
  const conversationName = useConversationName()
  const { data: customerServiceAssignees = [] } = useResource(
    resourceKeys.customerServiceAssignees(),
    () => listCustomerServiceAssignees(),
    { enabled: scope === InboxScope.InboxScopeCustomer },
  )

  const activeChatDraft =
    !targetIdentityId &&
    scope === InboxScope.InboxScopeInternal &&
    !selectedConversationId
      ? chatDraft
      : null
  useEffect(() => {
    if (selectedConversationId || scope !== InboxScope.InboxScopeInternal) {
      setChatDraft(null)
    }
  }, [scope, selectedConversationId])
  /** 选中一个会话。 */
  function selectConversation(conversationId: string) {
    navigationGeneration.current++
    setChatDraft(null)
    onSelectedConversationChange(conversationId)

    if (isNarrowViewport) {
      setIsNarrowDetailOpen(true)
    }
  }

  /** 不打开会话并把列表项推进到当前最后消息。 */
  async function markConversationAsRead(conversation: InboxConversation) {
    if (!conversation.lastMessageId) return
    await markConversationRead(conversation.id, {
      lastReadMessageId: conversation.lastMessageId,
      clearUnreadMark: true,
    })
  }

  /** 新建后先读取权威摘要，再将草稿连续切换为正式会话。 */
  async function showStartedConversation(conversation: InternalInboxConversationData) {
    const generation = navigationGeneration.current
    try {
      await summary.read(resourceKeys.conversationSummary(conversation.id), (signal) => readConversationSummary(conversation.id, signal))
    } catch (error) {
      console.warn("读取新建会话摘要失败", { conversationId: conversation.id, error })
    }
    void invalidate(resourceKeys.inbox())
    if (generation !== navigationGeneration.current) return
    setChatDraft(null)
    onQueryChange({ scope: InboxScope.InboxScopeInternal, conversationId: conversation.id, replace: Boolean(targetIdentityId) })
    setIsNarrowDetailOpen(isNarrowViewport)
  }

  /** 打开已有真人单聊，或在主区开始真人和 AI 聊天草稿。 */
  function showChatDraft(
    member: MemberOption,
    existing: DirectInboxConversationData | null = null,
  ) {
    if (existing) {
      showStartedConversation(existing)
      return
    }
    setChatDraft(
      member.type === OrganizationIdentityType.OrganizationIdentityTypeAgent
        ? { kind: "agent-draft", member, conversationId: crypto.randomUUID() }
        : { kind: "direct-draft", member },
    )
    onQueryChange({
      scope: InboxScope.InboxScopeInternal,
      conversationId: "",
    })
    setIsNarrowDetailOpen(isNarrowViewport)
  }

  /** 消息或客服处理保存后刷新列表与详情，保持当前筛选和选择。 */
  function refreshConversationAfterMessage(conversation: InboxConversation) {
    void invalidate(resourceKeys.inbox())
    void invalidate(resourceKeys.conversationSummary(conversation.id))
  }

  /** 主动退群后清空选择并关闭窄屏详情，不打开其他会话。 */
  function showConversationAfterGroupLeft(conversationID: string) {
    queue?.forgetConversation(conversationID)
    clearConversationResources(queryClient, conversationID)
    void queryClient.resetQueries({ queryKey: resourceKeys.conversationSummary(conversationID) })
    setChatDraft(null)
    setIsNarrowDetailOpen(false)
    onSelectedConversationChange("", true)
  }

  const selection: ConversationSelection | null = activeChatDraft
    ? activeChatDraft
    : selectedConversation
      ? { kind: "conversation", conversation: selectedConversation }
      : null

  const detailState = selectedConversationId && !selection ? (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
      {summary.loading ? <LoadingIndicator>{t("messagesLoading")}</LoadingIndicator> : (
        <>
          <p>{t(summary.data === null ? "conversationUnavailable" : "conversationLoadError")}</p>
          {summary.data !== null ? <Button variant="outline" size="sm" onClick={() => void summary.refresh()}>{t("common:actions.retry")}</Button> : null}
        </>
      )}
    </div>
  ) : null

  const pane = (
    <div className="flex min-h-0 flex-1 flex-col">
      <InboxPaneTop
        railCollapsed={railCollapsed}
        onRailToggle={() => setRailCollapsed((collapsed) => !collapsed)}
        onCreateGroup={() => setGroupDialogOpen(true)}
        onCreateAgent={() => setAgentDialogOpen(true)}
      />
      <div className="flex min-h-0 flex-1">
        {railCollapsed ? null : (
          <InboxScopeRail
            scope={scope}
            attentionUnreadCount={attentionUnreadCount}
            onScopeChange={(nextScope) => {
              setChatDraft(null)
              onQueryChange({ scope: nextScope })
            }}
          />
        )}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          {scope === InboxScope.InboxScopeCustomer ? (
            <InboxCustomerQueueFilter
              view={customerView}
              assigneeIdentityId={assigneeIdentityId}
              assignees={customerServiceAssignees}
              currentIdentityId={identity.user.identityId}
              onChange={(nextView, nextAssigneeIdentityId = "") =>
                onQueryChange({
                  customerView: nextView,
                  assigneeIdentityId: nextAssigneeIdentityId,
                })
              }
            />
          ) : null}
          <InboxListPanel list={list} viewport={listViewport} detailError={Boolean(summary.error)} retryDetail={() => void summary.refresh()}>
            <InboxConversationList
              conversations={conversations}
              onMenuChange={(open) => { listViewport.interaction.current.menu = open }}
              selectedId={selectedConversation?.id}
              onSelect={selectConversation}
              onMarkRead={markConversationAsRead}
            />
          </InboxListPanel>
        </div>
      </div>
    </div>
  )

  return (
    <>
      <PageSplit
        paneWidth={railCollapsed ? "inboxCollapsed" : "inbox"}
        paneOnNarrow="fill"
        className="bg-background"
        paneClassName="transition-[width]"
        pane={pane}
      >
        {isNarrowViewport ? null : targetIdentityId ? (
          <LoadingIndicator className="flex-1 justify-center">
            {t("chatTargetLoading")}
          </LoadingIndicator>
        ) : detailState ? detailState : selection ? (
          <section className="min-h-0 flex-1">
            <ConversationMain
              selection={selection}
              onSessionChanged={refreshConversationAfterMessage}
              onConversationChanged={refreshConversationAfterMessage}
              onGroupLeft={showConversationAfterGroupLeft}
              onChatStarted={showStartedConversation}
            />
          </section>
        ) : (
          <div className="cervi-inbox-empty-main flex min-h-0 flex-1 items-center justify-center p-6">
            <div
              data-slot="empty-state-content"
              className="max-w-sm text-center"
            >
              <div className="mx-auto mb-4 flex size-11 items-center justify-center rounded-xl border bg-background shadow-sm">
                <MessagesSquareIcon className="size-5 text-muted-foreground" />
              </div>
              <h2 className="text-base font-semibold tracking-tight">
                {t(hasConversations ? "selectConversationTitle" : "emptyTitle")}
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                {t(hasConversations ? "selectConversationDescription" : "emptyDescription")}
              </p>
            </div>
          </div>
        )}
      </PageSplit>

      {isNarrowViewport && (selection || detailState) ? (
        <Sheet
          open={isNarrowDetailOpen}
          onOpenChange={(open) => {
            setIsNarrowDetailOpen(open)
            if (!open) setChatDraft(null)
          }}
        >
          <SheetContent className="data-[side=right]:w-full p-0 sm:max-w-lg">
            <SheetHeader className="sr-only">
              <SheetTitle>
                {t("conversationTitle", {
                  name:
                    activeChatDraft?.member.displayName ??
                    (selectedConversation
                      ? conversationName(selectedConversation)
                      : ""),
                })}
              </SheetTitle>
              <SheetDescription>{t("detailDescription")}</SheetDescription>
            </SheetHeader>
            {selection ? (
            <ConversationMain
              selection={selection}
              onSessionChanged={refreshConversationAfterMessage}
              onConversationChanged={refreshConversationAfterMessage}
              onGroupLeft={showConversationAfterGroupLeft}
              onChatStarted={showStartedConversation}
              narrowViewport
            />
            ) : detailState}
          </SheetContent>
        </Sheet>
      ) : null}

      {targetIdentityId ? (
        <InboxConversationTarget
          key={targetIdentityId}
          identityId={targetIdentityId}
          currentIdentityId={identity.user.identityId}
          onSelected={showChatDraft}
          onFailed={() => {
            // 打开失败时结束旧草稿并回到内部会话列表。
            setChatDraft(null)
            onQueryChange({ conversationId: "" })
          }}
        />
      ) : null}
      <ConversationTargetPickerDialog
        open={agentDialogOpen}
        onOpenChange={setAgentDialogOpen}
        onSelected={showChatDraft}
      />
      <CreateGroupConversationDialog
        open={groupDialogOpen}
        currentIdentityID={identity.user.identityId}
        onOpenChange={setGroupDialogOpen}
        onCreated={showStartedConversation}
      />
    </>
  )
}
