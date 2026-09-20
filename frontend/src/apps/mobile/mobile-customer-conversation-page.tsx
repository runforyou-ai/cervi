/** 移动端客户会话详情、回复与客服处理周期操作。 */
import { LoaderCircleIcon, MoreHorizontalIcon, SearchIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useNavigate, useParams } from "react-router"

import {
  ChannelType,
  ConversationType,
  isCustomerInboxConversation,
  type CustomerInboxConversationData,
} from "@/api"
import { MobileIndividualThread } from "@/apps/mobile/mobile-individual-thread"
import {
  mobileSearchPath,
  useMobileNavigation,
  type MobileLocateState,
} from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  CustomerSessionCloseDialog,
  customerReplyDisabledReason,
  customerReplySupported,
  useCustomerSessionActions,
} from "@/features/inbox/customer-session-actions"
import { sessionStatusLabel } from "@/features/inbox/session-status-label"
import { useConversationAgentReplyLabel } from "@/features/inbox/conversation-agent-activity"
import { useConversationTypingLabel } from "@/features/inbox/use-conversation-typing"
import { useConversationSummary } from "@/features/inbox/use-conversation-summary"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 展示客服处理周期的领取、接管、转交、关闭与重新打开菜单。 */
function MobileCustomerSessionMenu({
  conversation,
}: {
  conversation: CustomerInboxConversationData
}) {
  const { t } = useTranslation("inbox")
  const { identity } = useMobileWorkspace()
  const invalidate = useResourceInvalidator()
  const actions = useCustomerSessionActions(
    conversation,
    identity.user.identityId,
    identity.user.handlesCustomers,
    () => {
      void invalidate(resourceKeys.inbox())
      void invalidate(resourceKeys.conversationSummary(conversation.id))
    },
  )
  const { operation, transferCandidates } = actions

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-lg"
            className="-mr-2"
            disabled={operation !== ""}
            aria-label={t("conversationMore")}
          >
            {operation ? (
              <LoaderCircleIcon className="animate-spin" />
            ) : (
              <MoreHorizontalIcon />
            )}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-48">
          {actions.reopenable ? (
            <DropdownMenuItem
              className="min-h-11"
              onSelect={() => void actions.reopen()}
            >
              {t("conversationReopen")}
            </DropdownMenuItem>
          ) : actions.claimable ? (
            <DropdownMenuItem
              className="min-h-11"
              onSelect={() => void actions.claim()}
            >
              {conversation.customer.assignee
                ? t("conversationTakeover")
                : t("conversationClaim")}
            </DropdownMenuItem>
          ) : !actions.transferable ? null : transferCandidates.length === 0 ? (
            <DropdownMenuItem className="min-h-11" disabled>
              {t("conversationTransferEmpty")}
            </DropdownMenuItem>
          ) : (
            transferCandidates.map((assignee) => (
              <DropdownMenuItem
                key={assignee.identityId}
                className="min-h-11"
                onSelect={() => void actions.transfer(assignee)}
              >
                {t("conversationTransferTo", { name: assignee.displayName })}
              </DropdownMenuItem>
            ))
          )}
          {actions.closable ? (
            <DropdownMenuItem
              className="min-h-11 text-destructive focus:text-destructive"
              onSelect={() => actions.setCloseConfirmationOpen(true)}
            >
              {t("conversationClose")}
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      <CustomerSessionCloseDialog actions={actions} />
    </>
  )
}

/** 加载客户会话摘要，展示历史、回复区、会话内搜索入口和处理菜单。 */
export function MobileCustomerConversationPage() {
  const { t } = useTranslation(["inbox", "common"])
  const { inboxURL } = useMobileNavigation()
  const { identity } = useMobileWorkspace()
  const { conversationID = "" } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const summary = useConversationSummary(conversationID, false)
  const locationState = location.state as
    | (MobileLocateState & { conversation?: CustomerInboxConversationData })
    | null
  // 路由携带的摘要保持首屏线程，查询完成后由服务端结果接管。
  const initial = locationState?.conversation
  const data =
    summary.data === undefined && initial?.id === conversationID
      ? initial
      : summary.data
  const conversation = data && isCustomerInboxConversation(data) ? data : null
  const typingLabel = useConversationTypingLabel(conversationID, null)
  const agentReplyLabel = useConversationAgentReplyLabel(conversationID)
  // 访客正在输入优先于 AI 员工正在回复。
  const activityLabel = typingLabel || agentReplyLabel
  if (!conversationID) return <Navigate to={inboxURL} replace />
  const customer = conversation?.customer
  const disabledReason = customer
    ? customerReplySupported(customer)
      ? customerReplyDisabledReason(
          customer,
          identity.user.identityId,
          identity.user.handlesCustomers,
          t,
        )
      : t("channelReplyUnsupported")
    : null

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobilePageHeader
        backTo={inboxURL}
        title={
          <span className="block min-w-0">
            <span className="block truncate">
              {customer
                ? (customer.contactName ?? t("anonymousVisitor"))
                : t("unknownSender")}
            </span>
            {customer ? (
              <span className="block truncate text-xs font-normal text-muted-foreground">
                {activityLabel ||
                  `${sessionStatusLabel(customer.serviceSessionStatus, t)} · ${customer.channelName}`}
              </span>
            ) : null}
          </span>
        }
        actions={
          <>
            <Button
              variant="ghost"
              size="icon-lg"
              aria-label={t("searchCurrentConversation")}
              disabled={!conversation}
              onClick={() =>
                void navigate(mobileSearchPath(conversationID), {
                  state: { mobileBack: true },
                })
              }
            >
              <SearchIcon />
            </Button>
            {conversation ? (
              <MobileCustomerSessionMenu conversation={conversation} />
            ) : null}
          </>
        }
      />
      {summary.loading && !conversation ? (
        <LoadingIndicator className="min-h-0 flex-1 justify-center">
          {t("messagesLoading")}
        </LoadingIndicator>
      ) : !conversation ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
          <p>
            {t(summary.error ? "conversationLoadError" : "conversationUnavailable")}
          </p>
          {summary.error ? (
            <Button
              variant="outline"
              className="min-h-11"
              onClick={() => void summary.refresh()}
            >
              {t("common:actions.retry")}
            </Button>
          ) : null}
        </div>
      ) : (
        <MobileIndividualThread
          key={conversationID}
          conversationID={conversationID}
          conversationType={ConversationType.ConversationTypeCustomer}
          customerDeliveries={
            conversation.customer.channelType === ChannelType.ChannelTypeTelegram
          }
          customerAttachment={conversation.customer}
          disabledReason={disabledReason}
          lastReadMessageID={conversation.lastReadMessageId}
          locateMessage={locationState?.locateMessage}
        />
      )}
    </section>
  )
}
