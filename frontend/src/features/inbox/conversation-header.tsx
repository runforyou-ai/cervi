/** 客户会话头与联系人头像。 */
import { useEffect, useState } from "react"
import {
  BotIcon,
  ChevronDownIcon,
  GlobeIcon,
  LoaderCircleIcon,
  MessageCircleIcon,
  MoreHorizontalIcon,
  SendIcon,
  UserRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ChannelType,
  CustomerInboxView,
  ServiceSessionStatus,
  claimServiceSession,
  closeServiceSession,
  isApiError,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  OrganizationIdentityType,
  listCustomerServiceAssignees,
  reopenServiceSession,
  transferServiceSession,
  type CustomerServiceSession,
  type InboxConversation,
} from "@/api"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

const sourceBadges: Partial<
  Record<ChannelType, { icon: typeof GlobeIcon; className: string }>
> = {
  [ChannelType.ChannelTypeWebsite]: {
    icon: GlobeIcon,
    className: "bg-badge-website",
  },
  [ChannelType.ChannelTypeTelegram]: {
    icon: SendIcon,
    className: "bg-badge-telegram",
  },
  [ChannelType.ChannelTypeWeChatOfficialAccount]: {
    icon: MessageCircleIcon,
    className: "bg-badge-wechat",
  },
}

/** 展示联系人头像和来源渠道角标。 */
export function ConversationAvatar({
  conversation,
  className,
}: {
  conversation: InboxConversation
  className?: string
}) {
  const customer = isCustomerInboxConversation(conversation)
    ? conversation.customer
    : null
  const direct = isDirectInboxConversation(conversation)
    ? conversation.direct
    : null
  const badge = customer ? sourceBadges[customer.channelType] : undefined
  const contactName = customer?.contactName?.trim() || direct?.peerName.trim()
  const avatarURL = customer?.contactAvatarUrl ?? ""
  const [avatarFailed, setAvatarFailed] = useState(false)
  const directAgent =
    direct?.peerType ===
    OrganizationIdentityType.OrganizationIdentityTypeAgent

  useEffect(() => setAvatarFailed(false), [avatarURL])

  return (
    <div className="relative shrink-0">
      <div
        className={cn(
          "flex size-10 items-center justify-center overflow-hidden rounded-lg bg-muted text-sm font-medium text-muted-foreground",
          className,
        )}
      >
        {avatarURL && !avatarFailed ? (
          <img
            src={avatarURL}
            alt=""
            className="size-full rounded-[inherit] object-cover"
            draggable={false}
            onError={() => setAvatarFailed(true)}
          />
        ) : directAgent ? (
          <BotIcon className="size-4.5" />
        ) : contactName ? (
          contactName.slice(0, 1).toLocaleUpperCase()
        ) : (
          <UserRoundIcon className="size-4.5" />
        )}
      </div>
      {badge ? (
        <span
          aria-hidden="true"
          className={cn(
            "absolute -right-0.5 -bottom-0.5 flex size-4 items-center justify-center rounded-full border-2 border-background text-white",
            badge.className,
          )}
        >
          <badge.icon className="size-2" />
        </span>
      ) : null}
    </div>
  )
}

/** 按 Helmdesk 会话头布局展示当前联系人、会话状态和操作区。 */
export function ConversationHeader({
  conversation,
  contactName,
  sessionStatus,
  currentIdentityId,
  onSessionMoved,
  contextVisible,
  narrowViewport = false,
  onContextToggle,
}: {
  conversation: InboxConversation
  contactName: string
  sessionStatus: string
  currentIdentityId: string
  onSessionMoved: (
    session: CustomerServiceSession,
    view: CustomerInboxView,
    assigneeIdentityId?: string,
  ) => void
  contextVisible: boolean
  narrowViewport?: boolean
  onContextToggle: () => void
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const [operation, setOperation] = useState("")
  const [closeConfirmationOpen, setCloseConfirmationOpen] = useState(false)
  const customer = isCustomerInboxConversation(conversation)
    ? conversation.customer
    : null
  const direct = isDirectInboxConversation(conversation)
    ? conversation.direct
    : null
  const agentRunLabel = agentRunStatusLabel(
    direct?.agentRunStatus ?? null,
    t,
  )
  const sessionOpen =
    customer?.serviceSessionStatus ===
    ServiceSessionStatus.ServiceSessionStatusOpen
  const sessionClosed =
    customer?.serviceSessionStatus ===
    ServiceSessionStatus.ServiceSessionStatusClosed
  const assignedToCurrentUser =
    customer?.assignee?.identityId === currentIdentityId
  const { data: assignees = [] } = useResource(
    resourceKeys.customerServiceAssignees(),
    () => listCustomerServiceAssignees(),
    { enabled: Boolean(customer && sessionOpen && assignedToCurrentUser) },
  )
  const transferCandidates = assignees.filter(
    (assignee) => assignee.identityId !== currentIdentityId,
  )
  const contextActionLabel = contextVisible
    ? t("contextClose")
    : t("contextOpen")

  /** 执行客服处理周期命令并通知上层刷新受影响视图。 */
  async function runSessionOperation(
    nextOperation: string,
    execute: () => Promise<CustomerServiceSession>,
    successMessage: string,
    afterSuccess?: (session: CustomerServiceSession) => void,
  ) {
    setOperation(nextOperation)
    try {
      const session = await execute()
      afterSuccess?.(session)
      toast.success(successMessage)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("更新客服会话失败", {
        conversationId: conversation.id,
        operation: nextOperation,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error)
          : t("conversationActionError"),
      )
    } finally {
      setOperation("")
    }
  }

  return (
    <>
      <header
        data-slot="conversation-header"
        className={cn(
          "flex shrink-0 items-center gap-3 border-b px-4 py-3",
          narrowViewport && "pr-14",
        )}
      >
      <ConversationAvatar
        conversation={conversation}
        className="size-9 rounded-full"
      />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <h2
            className="min-w-0 flex-1 truncate text-sm font-semibold"
            title={contactName}
          >
            {contactName}
          </h2>
        </div>
        {customer ? (
          <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
            <span className="inline-flex h-5 shrink-0 items-center rounded-md border px-1.5 text-[10px]">
              {sessionStatus}
            </span>
            <span
              className="inline-flex h-5 min-w-0 items-center truncate rounded-md border px-1.5 text-[10px]"
              title={customer.title}
            >
              {customer.title}
            </span>
          </div>
        ) : agentRunLabel ? (
          <div className="text-xs text-muted-foreground">
            {agentRunLabel}
          </div>
        ) : null}
      </div>
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
              onClick={() =>
                void runSessionOperation(
                  "reopen",
                  () => reopenServiceSession(conversation.id),
                  t("conversationReopenSuccess"),
                  (session) =>
                    onSessionMoved(
                      session,
                      CustomerInboxView.CustomerInboxViewMine,
                    ),
                )
              }
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
              onClick={() =>
                void runSessionOperation(
                  "claim",
                  () => claimServiceSession(conversation.id),
                  customer.assignee
                    ? t("conversationTakeoverSuccess")
                    : t("conversationClaimSuccess"),
                  (session) =>
                    onSessionMoved(
                      session,
                      CustomerInboxView.CustomerInboxViewMine,
                    ),
                )
              }
            >
              {operation === "claim" ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : null}
              {customer.assignee
                ? t("conversationTakeover")
                : t("conversationClaim")}
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
                      onSelect={() =>
                        void runSessionOperation(
                          `transfer:${assignee.identityId}`,
                          () =>
                            transferServiceSession(conversation.id, {
                              assigneeIdentityId: assignee.identityId,
                            }),
                          t("conversationTransferSuccess", {
                            name: assignee.displayName,
                          }),
                          (session) =>
                            onSessionMoved(
                              session,
                              CustomerInboxView.CustomerInboxViewCoworkers,
                              assignee.identityId,
                            ),
                        )
                      }
                    >
                      {assignee.displayName}
                    </DropdownMenuItem>
                  ))
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
          {sessionOpen &&
          (!customer.assignee || assignedToCurrentUser) ? (
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
                  onSelect={() => setCloseConfirmationOpen(true)}
                >
                  {t("conversationClose")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
          {customer ? (
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
                  <DropdownMenuItem
                    onSelect={() =>
                      void runSessionOperation(
                        "reopen",
                        () => reopenServiceSession(conversation.id),
                        t("conversationReopenSuccess"),
                        (session) =>
                          onSessionMoved(
                            session,
                            CustomerInboxView.CustomerInboxViewMine,
                          ),
                      )
                    }
                  >
                    {t("conversationReopen")}
                  </DropdownMenuItem>
                ) : !assignedToCurrentUser ? (
                  <DropdownMenuItem
                    onSelect={() =>
                      void runSessionOperation(
                        "claim",
                        () => claimServiceSession(conversation.id),
                        customer.assignee
                          ? t("conversationTakeoverSuccess")
                          : t("conversationClaimSuccess"),
                        (session) =>
                          onSessionMoved(
                            session,
                            CustomerInboxView.CustomerInboxViewMine,
                          ),
                      )
                    }
                  >
                    {customer.assignee
                      ? t("conversationTakeover")
                      : t("conversationClaim")}
                  </DropdownMenuItem>
                ) : (
                  transferCandidates.map((assignee) => (
                    <DropdownMenuItem
                      key={assignee.identityId}
                      onSelect={() =>
                        void runSessionOperation(
                          `transfer:${assignee.identityId}`,
                          () =>
                            transferServiceSession(conversation.id, {
                              assigneeIdentityId: assignee.identityId,
                            }),
                          t("conversationTransferSuccess", {
                            name: assignee.displayName,
                          }),
                          (session) =>
                            onSessionMoved(
                              session,
                              CustomerInboxView.CustomerInboxViewCoworkers,
                              assignee.identityId,
                            ),
                        )
                      }
                    >
                      {t("conversationTransferTo", {
                        name: assignee.displayName,
                      })}
                    </DropdownMenuItem>
                  ))
                )}
                {sessionOpen &&
                (!customer.assignee || assignedToCurrentUser) ? (
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onSelect={() => setCloseConfirmationOpen(true)}
                  >
                    {t("conversationClose")}
                  </DropdownMenuItem>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="text-muted-foreground xl:hidden"
            aria-label={contextActionLabel}
            aria-pressed={contextVisible}
            title={contextActionLabel}
            onClick={onContextToggle}
          >
            {t("contextTitleBar")}
          </Button>
        </div>
      ) : null}
      </header>
      <AlertDialog
        open={closeConfirmationOpen}
        onOpenChange={setCloseConfirmationOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("conversationCloseConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("conversationCloseConfirmDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("conversationCloseCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() =>
                void runSessionOperation(
                  "close",
                  () => closeServiceSession(conversation.id),
                  t("conversationCloseSuccess"),
                  (session) =>
                    onSessionMoved(
                      session,
                      CustomerInboxView.CustomerInboxViewClosed,
                    ),
                )
              }
            >
              {t("conversationCloseConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
