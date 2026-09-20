/** 成员会话头与图标操作区。 */
import {
  ArchiveIcon,
  ArrowRightLeftIcon,
  LoaderCircleIcon,
  MoreVerticalIcon,
  RotateCcwIcon,
  SearchIcon,
  UserRoundPlusIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  ConversationStatus,
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  UserStatus,
  type GroupParticipant,
  type InboxConversation,
} from "@/api"
import { Button } from "@/components/ui/button"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import {
  CustomerSessionCloseDialog,
  useCustomerSessionActions,
} from "@/features/inbox/customer-session-actions"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { useConversationAgentReplyLabel } from "@/features/inbox/conversation-agent-activity"
import { useConversationTypingLabel } from "@/features/inbox/use-conversation-typing"
import { workStatusLabel } from "@/components/work-status"
import { cn } from "@/lib/utils"

/** 会话头的图标操作按钮，悬停显示操作名称。 */
export function HeaderAction({
  label,
  icon: Icon,
  busy = false,
  disabled = false,
  destructive = false,
  className,
  onClick,
}: {
  label: string
  icon: typeof SearchIcon
  busy?: boolean
  disabled?: boolean
  destructive?: boolean
  className?: string
  onClick: () => void
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className={cn(
            "shrink-0",
            destructive
              ? "text-destructive hover:bg-destructive/10 hover:text-destructive"
              : "text-muted-foreground",
            className,
          )}
          aria-label={label}
          disabled={disabled}
          onClick={onClick}
        >
          {busy ? <LoaderCircleIcon className="animate-spin" /> : <Icon />}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}

/** 按 Helmdesk 会话头布局展示当前联系人、会话状态和操作区。 */
export function ConversationHeader({
  conversation,
  contactName,
  sessionStatus,
  currentIdentityId,
  onSessionChanged,
  onSearch,
  groupParticipants,
  narrowViewport = false,
  contextVisible = false,
  onToggleContext,
}: {
  conversation: InboxConversation
  contactName: string
  sessionStatus: string
  currentIdentityId: string
  onSessionChanged: () => void
  onSearch?: () => void
  groupParticipants?: GroupParticipant[]
  narrowViewport?: boolean
  contextVisible?: boolean
  onToggleContext?: () => void
}) {
  const { t } = useTranslation(["inbox", "common"])
  const { t: tCommon } = useTranslation("common")
  const customerConversation = isCustomerInboxConversation(conversation)
    ? conversation
    : null
  const customer = customerConversation?.customer ?? null
  const agent = isAgentInboxConversation(conversation) ? conversation.agent : null
  const direct = isDirectInboxConversation(conversation) ? conversation.direct : null
  const group = isGroupInboxConversation(conversation) ? conversation.group : null
  const agentRunLabel = agentRunStatusLabel(agent?.agentRunStatus ?? null, t)
  // 正在输入提示占用副标题位置：群聊替换人数，单聊替换对方工作状态，客户会话补在状态徽章之后。
  const typingLabel = useConversationTypingLabel(
    conversation.id,
    group ? (groupParticipants ?? []) : null,
  )
  const agentReplyLabel = useConversationAgentReplyLabel(conversation.id)
  // 真人正在输入优先于 AI 员工正在回复。
  const activityLabel = typingLabel || agentReplyLabel
  const actions = useCustomerSessionActions(
    customerConversation,
    currentIdentityId,
    onSessionChanged,
  )
  const {
    operation,
    sessionOpen,
    sessionClosed,
    assignedToCurrentUser,
    transferCandidates,
  } = actions

  return (
    <>
      <header
        data-slot="conversation-header"
        className={cn(
          "flex min-h-12 shrink-0 items-center gap-2.5 px-3 py-2",
          narrowViewport && "pr-14",
        )}
      >
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <Tooltip>
              <TooltipTrigger asChild>
                <h2
                  data-slot="conversation-header-title"
                  className="min-w-0 truncate text-sm font-semibold"
                >
                  {contactName}
                </h2>
              </TooltipTrigger>
              <TooltipContent className="max-w-80">{contactName}</TooltipContent>
            </Tooltip>
            {activityLabel ? (
              <span className="shrink-0 text-xs text-muted-foreground">
                {activityLabel}
              </span>
            ) : null}
          </div>
          {customer ? (
            <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
              <span
                data-slot="conversation-header-detail"
                className="inline-flex h-5 shrink-0 items-center rounded-md border px-1.5 text-[10px]"
              >
                {sessionStatus}
              </span>
              <span
                data-slot="conversation-header-detail"
                className="inline-flex h-5 min-w-0 items-center truncate rounded-md border px-1.5 text-[10px]"
                title={customer.title}
              >
                {customer.title}
              </span>
            </div>
          ) : group ? (
            <p
              data-slot="conversation-header-detail"
              className="w-fit max-w-full truncate text-xs text-muted-foreground"
            >
              {group.status === ConversationStatus.ConversationStatusArchived
                ? t("groupDissolved")
                : t("groupMemberCount", { count: group.memberCount })}
            </p>
          ) : direct ? (
            <p
              data-slot="conversation-header-detail"
              className="w-fit max-w-full truncate text-xs text-muted-foreground"
            >
              {direct.peerStatus === UserStatus.UserStatusInactive
                ? t("directPeerDisabled")
                : workStatusLabel(direct.peerWorkStatus, tCommon)}
            </p>
          ) : agentRunLabel ? (
            <p
              data-slot="conversation-header-detail"
              className="w-fit max-w-full truncate text-xs text-muted-foreground"
            >
              {agentRunLabel}
            </p>
          ) : null}
        </div>
        <div
          data-slot="conversation-actions"
          className="flex shrink-0 items-center gap-0.5"
        >
          {onSearch ? (
            <HeaderAction
              label={t("searchCurrentConversation")}
              icon={SearchIcon}
              onClick={onSearch}
            />
          ) : null}
          {customer && sessionClosed ? (
            <HeaderAction
              label={t("conversationReopen")}
              icon={RotateCcwIcon}
              busy={operation === "reopen"}
              disabled={operation !== ""}
              onClick={() => void actions.reopen()}
            />
          ) : null}
          {customer && sessionOpen && !assignedToCurrentUser ? (
            <HeaderAction
              label={
                customer.assignee
                  ? t("conversationTakeover")
                  : t("conversationClaim")
              }
              icon={UserRoundPlusIcon}
              busy={operation === "claim"}
              disabled={operation !== ""}
              onClick={() => void actions.claim()}
            />
          ) : null}
          {customer && sessionOpen && assignedToCurrentUser ? (
            <DropdownMenu>
              <Tooltip>
                <TooltipTrigger asChild>
                  <DropdownMenuTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      className="shrink-0 text-muted-foreground"
                      disabled={operation !== ""}
                      aria-label={t("conversationTransfer")}
                    >
                      {operation.startsWith("transfer:") ? (
                        <LoaderCircleIcon className="animate-spin" />
                      ) : (
                        <ArrowRightLeftIcon />
                      )}
                    </Button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent>{t("conversationTransfer")}</TooltipContent>
              </Tooltip>
              <DropdownMenuContent align="end" className="min-w-48">
                {transferCandidates.length === 0 ? (
                  <DropdownMenuItem disabled>
                    {t("conversationTransferEmpty")}
                  </DropdownMenuItem>
                ) : (
                  transferCandidates.map((assignee) => (
                    <DropdownMenuItem
                      key={assignee.identityId}
                      onSelect={() => void actions.transfer(assignee)}
                    >
                      {assignee.displayName}
                    </DropdownMenuItem>
                  ))
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
          {customer && actions.closable ? (
            <HeaderAction
              label={t("conversationClose")}
              icon={ArchiveIcon}
              destructive
              busy={operation === "close"}
              disabled={operation !== ""}
              onClick={() => actions.setCloseConfirmationOpen(true)}
            />
          ) : null}
          {onToggleContext ? (
            <HeaderAction
              label={contextVisible ? t("contextClose") : t("contextOpen")}
              icon={MoreVerticalIcon}
              onClick={onToggleContext}
            />
          ) : null}
        </div>
      </header>
      <CustomerSessionCloseDialog actions={actions} />
    </>
  )
}
