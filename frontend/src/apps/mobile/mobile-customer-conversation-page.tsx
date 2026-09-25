/** 移动端客户会话详情、回复、客户语言、AI 助手、客户资料与业务入口及客服处理周期操作。 */
import { useEffect, useRef, useState, type RefObject } from "react"
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
  ServiceSessionStatus,
  isCustomerInboxConversation,
  type CustomerInboxConversationData,
} from "@/api"
import { MobileCustomerConversationTitle } from "@/apps/mobile/mobile-customer-conversation-title"
import { MobileCustomerTransferSheet } from "@/apps/mobile/mobile-customer-transfer-sheet"
import { MobileIndividualThread } from "@/apps/mobile/mobile-individual-thread"
import {
  mobileSearchPath,
  useMobileBack,
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
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { CustomerTranslationProvider } from "@/features/inbox/customer-translation"
import type { ComposerDraftBridge } from "@/features/inbox/conversation-composer-types"
import { HandoffSummaryCard } from "@/features/inbox/handoff-summary-card"
import {
  CustomerSessionCloseDialog,
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

/** 客户会话页向 AI 助手、客户资料与业务子页提供的会话、对客草稿入口和回复限制。 */
export type MobileCustomerConversationContext = {
  conversation: CustomerInboxConversationData
  customerDraftRef: RefObject<ComposerDraftBridge | null>
  replyDisabledReason: string | null
}

/** 展示客户资料、业务、会话内搜索及客服处理周期的领取、接管、转交、关闭与重新打开菜单；转交在底部面板中选择去向，关闭成功后返回来源列表。 */
function MobileCustomerSessionMenu({
  conversation,
}: {
  conversation: CustomerInboxConversationData
}) {
  const { t } = useTranslation("inbox")
  const { identity } = useMobileWorkspace()
  const { inboxURL } = useMobileNavigation()
  const back = useMobileBack(inboxURL)
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const alive = useRef(true)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])
  const actions = useCustomerSessionActions(
    conversation,
    identity.user.identityId,
    identity.user.handlesCustomers,
    (session) => {
      void invalidate(resourceKeys.inbox())
      void invalidate(resourceKeys.conversationSummary(conversation.id))
      // 离开会话页后到达的关闭结果只刷新数据，不再导航。
      if (alive.current && session.status === ServiceSessionStatus.ServiceSessionStatusClosed) back()
    },
  )
  const { operation } = actions
  const [transferOpen, setTransferOpen] = useState(false)
  const menuTriggerRef = useRef<HTMLButtonElement>(null)

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            ref={menuTriggerRef}
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
              void navigate(`/inbox/customer/${conversation.id}/profile`, {
                state: { conversation, mobileBack: true },
              })
            }
          >
            {t("customerProfile")}
          </DropdownMenuItem>
          <DropdownMenuItem
            className="min-h-11"
            onSelect={() =>
              void navigate(`/inbox/customer/${conversation.id}/business`, {
                state: { conversation, mobileBack: true },
              })
            }
          >
            {t("contextBusinessTab")}
          </DropdownMenuItem>
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
          ) : actions.transferable ? (
            <DropdownMenuItem
              className="min-h-11"
              onSelect={() => setTransferOpen(true)}
            >
              {t("conversationTransfer")}
            </DropdownMenuItem>
          ) : null}
          {actions.closable ? <DropdownMenuSeparator /> : null}
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
      <MobileCustomerTransferSheet
        actions={actions}
        open={transferOpen}
        onOpenChange={setTransferOpen}
        returnFocusRef={menuTriggerRef}
      />
      <CustomerSessionCloseDialog actions={actions} />
    </>
  )
}

/** 加载客户会话摘要，展示历史、回复区、标题下方的输入状态与客户语言、AI 助手入口和处理菜单；子页打开时保留会话与草稿。 */
export function MobileCustomerConversationPage() {
  const { t } = useTranslation(["inbox", "common"])
  const { inboxURL } = useMobileNavigation()
  const { identity } = useMobileWorkspace()
  const { conversationID = "" } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const childOpen = !useMatch("/inbox/customer/:conversationID")
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

  const covered = childOpen && Boolean(conversation)

  return (
    <CustomerTranslationProvider key={conversationID} conversationID={conversation ? conversationID : null}>
    <div className="relative h-full min-h-0">
      <section
        className={`flex h-full min-h-0 flex-col bg-background ${covered ? "absolute inset-0 opacity-0 pointer-events-none" : ""}`}
        inert={covered}
      >
        <MobilePageHeader
          backTo={covered ? undefined : inboxURL}
          title={
            <MobileCustomerConversationTitle
              name={conversation ? conversationName(conversation) : t("unknownSender")}
              activityLabel={activityLabel || null}
            />
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
          <>
          <HandoffSummaryCard
            key={`handoff-${conversationID}`}
            conversationID={conversationID}
            assignee={conversation.customer.assignee}
          />
          <MobileIndividualThread
            key={conversationID}
            conversationID={conversationID}
            conversationType={ConversationType.ConversationTypeCustomer}
            customerDeliveries={
              conversation.customer.channelType === ChannelType.ChannelTypeTelegram
            }
            customerAttachment={conversation.customer}
            disabledReason={disabledReason}
            enabled={!childOpen}
            customerDraftRef={customerDraftRef}
            lastReadMessageID={conversation.lastReadMessageId}
            locateMessage={locationState?.locateMessage}
          />
          </>
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
            } satisfies MobileCustomerConversationContext
          }
        />
      ) : null}
    </div>
    </CustomerTranslationProvider>
  )
}
