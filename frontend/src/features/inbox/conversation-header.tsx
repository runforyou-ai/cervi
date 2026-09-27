/** 成员会话头与图标操作区。 */
import {
  ArchiveIcon,
  ArrowRightLeftIcon,
  LoaderCircleIcon,
  PanelRightOpenIcon,
  RotateCcwIcon,
  SearchIcon,
  UserRoundCheckIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  OrganizationIdentityType,
  isAgentInboxConversation,
  isServiceInboxConversation,
  isGroupInboxConversation,
  type GroupParticipant,
  type InboxConversationData,
} from "@/api"
import { Button } from "@/components/ui/button"
import {
  CustomerSessionCloseDialog,
  CustomerTransferMenuItems,
  useCustomerSessionActions,
} from "@/features/inbox/customer-session-actions"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { assistantPresenceLabel } from "@/features/inbox/agent-run-status"
import { useAssistantDisplayName } from "@/hooks/use-assistant-display-name"
import {
  customerTypingSenderName,
  groupTypingSenderName,
  useConversationTypingLabel,
} from "@/features/inbox/use-conversation-typing"
import { cn } from "@/lib/utils"
import { CustomerLanguageChip } from "@/features/inbox/customer-language-menu"

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
  conversation: InboxConversationData
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
  const assistantDisplayName = useAssistantDisplayName()
  const customerConversation = isServiceInboxConversation(conversation)
    ? conversation
    : null
  const customer = customerConversation?.service ?? null
  const group = isGroupInboxConversation(conversation) ? conversation.group : null
  const assistant =
    isAgentInboxConversation(conversation) &&
    conversation.agent.agentType === OrganizationIdentityType.OrganizationIdentityTypeAssistant
      ? conversation.agent
      : null
  // 助理不在线时在标题旁说明原因，正常在线不额外提示。
  const presenceLabel = assistantPresenceLabel(assistant?.assistantPresence, t)
  // 正在输入提示紧接标题右侧展示，真人与 AI 员工同等列出。
  const activityLabel = useConversationTypingLabel(
    conversation.id,
    group
      ? groupTypingSenderName(groupParticipants ?? [], assistantDisplayName)
      : customer
        ? customerTypingSenderName(customer)
        : null,
  )
  const actions = useCustomerSessionActions(
    customerConversation,
    currentIdentityId,
    handlesCustomers,
    onSessionChanged,
  )
  const { operation } = actions

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
                className="min-w-0 truncate text-xl font-semibold"
              >
                {contactName}
              </h2>
            </TooltipTrigger>
            <TooltipContent className="max-w-80">{contactName}</TooltipContent>
          </Tooltip>
          {customer ? <CustomerLanguageChip /> : null}
          {(activityLabel ?? presenceLabel) ? (
            <span className="shrink-0 text-xs text-muted-foreground">
              {activityLabel ?? presenceLabel}
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
              icon={UserRoundCheckIcon}
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
                <CustomerTransferMenuItems actions={actions} />
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
          {onToggleContext && !contextVisible ? (
            <HeaderAction
              label={t("sidePanelOpen")}
              icon={PanelRightOpenIcon}
              onClick={onToggleContext}
            />
          ) : null}
        </div>
      </header>
      <CustomerSessionCloseDialog actions={actions} />
    </>
  )
}
