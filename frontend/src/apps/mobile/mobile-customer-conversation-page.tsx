/** 移动端客户会话详情、回复、AI 助手入口与客服处理周期操作。 */
import { useRef } from "react"
import {
  LoaderCircleIcon,
  MoreHorizontalIcon,
  SparklesIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Navigate,
  Outlet,
  useLocation,
  useMatch,
  useNavigate,
  useParams,
} from "react-router"

import {
  ChannelType,
  ConversationType,
  isCustomerInboxConversation,
  type CustomerInboxConversationData,
} from "@/api"
import type { MobileCustomerCopilotContext } from "@/apps/mobile/mobile-customer-copilot-page"
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
import type { ComposerDraftBridge } from "@/features/inbox/conversation-composer"
import {
  CustomerSessionCloseDialog,
  CustomerTransferMenuItems,
  customerReplyDisabledReason,
  customerReplySupported,
  useCustomerSessionActions,
} from "@/features/inbox/customer-session-actions"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import {
  customerTypingSenderName,
  useConversationTypingLabel,
} from "@/features/inbox/use-conversation-typing"
import { useConversationSummary } from "@/features/inbox/use-conversation-summary"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 展示会话内搜索及客服处理周期的领取、接管、转交、关闭与重新打开菜单。 */
function MobileCustomerSessionMenu({
  conversation,
}: {
  conversation: CustomerInboxConversationData
}) {
  const { t } = useTranslation("inbox")
  const { identity } = useMobileWorkspace()
  const navigate = useNavigate()
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
  const { operation } = actions

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
          <DropdownMenuItem
            className="min-h-11"
            onSelect={() =>
              void navigate(mobileSearchPath(conversation.id), {
                state: { mobileBack: true },
              })
            }
          >
            {t("searchCurrentConversation")}
          </DropdownMenuItem>
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
          ) : !actions.transferable ? null : (
            <CustomerTransferMenuItems actions={actions} itemClassName="min-h-11" />
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

/** 加载客户会话摘要，展示历史、回复区、AI 助手入口和包含会话内搜索的处理菜单；AI 助手子页打开时保留会话与草稿。 */
export function MobileCustomerConversationPage() {
  const { t } = useTranslation(["inbox", "common"])
  const { inboxURL } = useMobileNavigation()
  const { identity } = useMobileWorkspace()
  const { conversationID = "" } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const copilotOpen = !useMatch("/inbox/customer/:conversationID")
  const customerDraftRef = useRef<ComposerDraftBridge | null>(null)
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
  const activityLabel = useConversationTypingLabel(
    conversationID,
    conversation ? customerTypingSenderName(conversation.customer) : null,
  )
  const conversationName = useConversationName()
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

  const covered = copilotOpen && Boolean(conversation)

  return (
    <div className="relative h-full min-h-0">
      <section
        className={`flex h-full min-h-0 flex-col bg-background ${covered ? "absolute inset-0 opacity-0 pointer-events-none" : ""}`}
        inert={covered}
      >
        <MobilePageHeader
          backTo={covered ? undefined : inboxURL}
          title={
            <span className="block min-w-0 truncate">
              {activityLabel ||
                (conversation ? conversationName(conversation) : t("unknownSender"))}
            </span>
          }
          actions={
            <>
              <Button
                variant="ghost"
                size="icon-lg"
                aria-label={t("contextAssistantTab")}
                title={t("contextAssistantTab")}
                disabled={!conversation}
                onClick={() =>
                  void navigate(`/inbox/customer/${conversationID}/copilot`, {
                    state: { conversation, mobileBack: true },
                  })
                }
              >
                <SparklesIcon />
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
            enabled={!copilotOpen}
            customerDraftRef={customerDraftRef}
            lastReadMessageID={conversation.lastReadMessageId}
            locateMessage={locationState?.locateMessage}
          />
        )}
      </section>
      {conversation ? (
        <Outlet
          key={conversation.id}
          context={
            {
              conversation,
              customerDraftRef,
              replyDisabledReason: disabledReason,
            } satisfies MobileCustomerCopilotContext
          }
        />
      ) : null}
    </div>
  )
}
