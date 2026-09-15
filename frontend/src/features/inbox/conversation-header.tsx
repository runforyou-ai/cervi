/** 成员会话头与操作菜单。 */
import { ChevronDownIcon, LoaderCircleIcon, MoreHorizontalIcon, SearchIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  ConversationStatus,
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isGroupInboxConversation,
  type InboxConversation,
} from "@/api"
import { Button } from "@/components/ui/button"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
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
import { cn } from "@/lib/utils"

/** 按 Helmdesk 会话头布局展示当前联系人、会话状态和操作区。 */
export function ConversationHeader({
  conversation,
  contactName,
  sessionStatus,
  currentIdentityId,
  onSessionChanged,
  onSearch,
  narrowViewport = false,
}: {
  conversation: InboxConversation
  contactName: string
  sessionStatus: string
  currentIdentityId: string
  onSessionChanged: () => void
  onSearch: () => void
  narrowViewport?: boolean
}) {
  const { t } = useTranslation(["inbox", "common"])
  const customerConversation = isCustomerInboxConversation(conversation)
    ? conversation
    : null
  const customer = customerConversation?.customer ?? null
  const agent = isAgentInboxConversation(conversation) ? conversation.agent : null
  const group = isGroupInboxConversation(conversation) ? conversation.group : null
  const agentRunLabel = agentRunStatusLabel(agent?.agentRunStatus ?? null, t)
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
          "flex shrink-0 items-center gap-3 border-b px-4 py-3",
          narrowViewport && "pr-14",
        )}
      >
        <ConversationAvatar conversation={conversation} className="size-9" />
        <div className="min-w-0 flex-1">
          <div className="w-fit max-w-full">
            <h2
              data-slot="conversation-header-title"
              className="w-fit max-w-full truncate text-sm font-semibold"
              title={contactName}
            >
              {contactName}
            </h2>
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
                className="w-fit max-w-full text-xs text-muted-foreground"
              >
                {group.status === ConversationStatus.ConversationStatusArchived
                  ? t("groupDissolved")
                  : t("groupMemberCount", { count: group.memberCount })}
              </p>
            ) : agent ? (
              <div
                data-slot="conversation-header-detail"
                className="w-fit max-w-full text-xs text-muted-foreground"
              >
                {agent.agentName}
                {agentRunLabel ? ` · ${agentRunLabel}` : ""}
              </div>
            ) : null}
          </div>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="shrink-0 text-muted-foreground"
          aria-label={t("searchCurrentConversation")}
          title={t("searchCurrentConversation")}
          onClick={onSearch}
        >
          <SearchIcon />
        </Button>
        {customer ? (
          <div
            data-slot="conversation-actions"
            className="flex shrink-0 items-center gap-2"
          >
            {sessionClosed ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="hidden lg:inline-flex"
                disabled={operation !== ""}
                onClick={() => void actions.reopen()}
              >
                {operation === "reopen" ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {t("conversationReopen")}
              </Button>
            ) : null}
            {sessionOpen && !assignedToCurrentUser ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="hidden lg:inline-flex"
                disabled={operation !== ""}
                onClick={() => void actions.claim()}
              >
                {operation === "claim" ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {customer.assignee ? t("conversationTakeover") : t("conversationClaim")}
              </Button>
            ) : null}
            {sessionOpen && assignedToCurrentUser ? (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="hidden lg:inline-flex"
                    disabled={operation !== ""}
                  >
                    {operation.startsWith("transfer:") ? (
                      <LoaderCircleIcon className="animate-spin" />
                    ) : null}
                    {t("conversationTransfer")}
                    <ChevronDownIcon />
                  </Button>
                </DropdownMenuTrigger>
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
            {actions.closable ? (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="hidden lg:inline-flex"
                    disabled={operation !== ""}
                    aria-label={t("conversationMore")}
                    title={t("conversationMore")}
                  >
                    {operation === "close" ? (
                      <LoaderCircleIcon className="animate-spin" />
                    ) : (
                      <MoreHorizontalIcon />
                    )}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onSelect={() => actions.setCloseConfirmationOpen(true)}
                  >
                    {t("conversationClose")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            ) : null}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="lg:hidden"
                  disabled={operation !== ""}
                  aria-label={t("conversationMore")}
                  title={t("conversationMore")}
                >
                  {operation ? (
                    <LoaderCircleIcon className="animate-spin" />
                  ) : (
                    <MoreHorizontalIcon />
                  )}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-48">
                {sessionClosed ? (
                  <DropdownMenuItem onSelect={() => void actions.reopen()}>
                    {t("conversationReopen")}
                  </DropdownMenuItem>
                ) : !assignedToCurrentUser ? (
                  <DropdownMenuItem onSelect={() => void actions.claim()}>
                    {customer.assignee
                      ? t("conversationTakeover")
                      : t("conversationClaim")}
                  </DropdownMenuItem>
                ) : transferCandidates.length === 0 ? (
                  <DropdownMenuItem disabled>
                    {t("conversationTransferEmpty")}
                  </DropdownMenuItem>
                ) : (
                  transferCandidates.map((assignee) => (
                    <DropdownMenuItem
                      key={assignee.identityId}
                      onSelect={() => void actions.transfer(assignee)}
                    >
                      {t("conversationTransferTo", {
                        name: assignee.displayName,
                      })}
                    </DropdownMenuItem>
                  ))
                )}
                {actions.closable ? (
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onSelect={() => actions.setCloseConfirmationOpen(true)}
                  >
                    {t("conversationClose")}
                  </DropdownMenuItem>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        ) : null}
      </header>
      <CustomerSessionCloseDialog actions={actions} />
    </>
  )
}
