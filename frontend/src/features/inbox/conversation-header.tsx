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
  isCustomerInboxConversation,
  isGroupInboxConversation,
  type GroupParticipant,
  type InboxConversation,
} from "@/api"
import { Button } from "@/components/ui/button"
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

/** 展示当前会话名称、正在输入状态和操作区。 */
export function ConversationHeader({
  conversation,
  contactName,
  currentIdentityId,
  handlesCustomers,
  onSessionChanged,
  onSearch,
  groupParticipants,
  narrowViewport = false,
  contextVisible = false,
  onToggleContext,
}: {
  conversation: InboxConversation
  contactName: string
  currentIdentityId: string
  handlesCustomers: boolean
  onSessionChanged: () => void
  onSearch?: () => void
  groupParticipants?: GroupParticipant[]
  narrowViewport?: boolean
  contextVisible?: boolean
  onToggleContext?: () => void
}) {
  const { t } = useTranslation(["inbox", "common"])
  const customerConversation = isCustomerInboxConversation(conversation)
    ? conversation
    : null
  const customer = customerConversation?.customer ?? null
  const group = isGroupInboxConversation(conversation) ? conversation.group : null
  // 正在输入提示紧接标题右侧展示。
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
    handlesCustomers,
    onSessionChanged,
  )
  const { operation, transferCandidates } = actions

  return (
    <>
      <header
        data-slot="conversation-header"
        className={cn(
          "flex min-h-12 shrink-0 items-center gap-2.5 px-3 py-2",
          narrowViewport && "pr-14",
        )}
      >
        {/* 标题只占文字宽度，右侧留白保持窗口可拖动。 */}
        <div className="flex min-w-0 flex-1 items-center gap-2">
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
          {customer && actions.reopenable ? (
            <HeaderAction
              label={t("conversationReopen")}
              icon={RotateCcwIcon}
              busy={operation === "reopen"}
              disabled={operation !== ""}
              onClick={() => void actions.reopen()}
            />
          ) : null}
          {customer && actions.claimable ? (
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
          {customer && actions.transferable ? (
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
