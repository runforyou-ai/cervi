/** 客服、单聊与群聊共用的会话资料栏。 */
import {
  BotIcon,
  BriefcaseBusinessIcon,
  PanelRightCloseIcon,
} from "lucide-react"
import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type RefObject } from "react"
import { useTranslation } from "react-i18next"

import {
  OrganizationIdentityType,
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type InboxConversation,
  type MemberOption,
} from "@/api"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import { DirectConversationDraftAvatar } from "@/features/inbox/direct-conversation-draft-header"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import type { ComposerDraftBridge } from "@/features/inbox/conversation-composer"
import { CustomerCopilotPanel } from "@/features/inbox/customer-copilot-panel"
import { GroupConversationContext } from "@/features/inbox/group-conversation-context"
import { HeaderAction } from "@/features/inbox/conversation-header"
import { cn } from "@/lib/utils"

const contextPanelMinWidth = 320
const contextPanelDefaultWidth = 384
const contextPanelMaxWidth = 640
// 展开资料栏后为会话区保留的宽度，空间不足时资料栏最多占一半。
const conversationMinWidth = 420

/** 展示尚无数据的上下文页签。 */
function ContextPlaceholder({
  icon: Icon,
  title,
  description,
}: {
  icon: typeof BotIcon
  title: string
  description: string
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center px-6 text-center">
      <div className="mb-3 flex size-10 items-center justify-center rounded-xl border bg-muted/30 text-muted-foreground">
        <Icon className="size-4" />
      </div>
      <h3 className="text-sm font-medium">{title}</h3>
      <p className="mt-1.5 max-w-60 text-xs leading-5 text-muted-foreground">
        {description}
      </p>
    </div>
  )
}

/** 展示单聊的基础资料。 */
function InternalConversationProfile({
  conversation,
  directTarget,
  displayName,
}: {
  conversation: InboxConversation | null
  directTarget: MemberOption | null
  displayName: string
}) {
  const { t } = useTranslation("inbox")
  const direct =
    conversation && isDirectInboxConversation(conversation)
      ? conversation.direct
      : null
  const agent =
    conversation && isAgentInboxConversation(conversation)
      ? conversation.agent
      : null
  const identityType =
    agent ||
    (direct?.peerType ?? directTarget?.type) ===
      OrganizationIdentityType.OrganizationIdentityTypeAgent
      ? t("contextIdentityAgent")
      : t("contextIdentityMember")
  const agentStatus = agentRunStatusLabel(agent?.agentRunStatus ?? null, t)

  return (
    <dl className="space-y-1 text-sm">
      <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
        <dt className="flex min-h-7 items-center text-xs text-muted-foreground">
          {t("contextContactName")}
        </dt>
        <dd className="flex min-h-7 min-w-0 items-center gap-2">
          {conversation ? (
            <ConversationAvatar
              conversation={conversation}
              className="size-7 text-xs"
            />
          ) : directTarget ? (
            <DirectConversationDraftAvatar
              member={directTarget}
              className="size-7 text-xs"
            />
          ) : null}
          <span
            className="min-w-0 truncate"
            title={agent?.agentName ?? displayName}
          >
            {agent?.agentName ?? displayName}
          </span>
        </dd>
      </div>
      {direct || agent || directTarget ? (
        <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
          <dt className="flex min-h-7 items-center text-xs text-muted-foreground">
            {t("contextIdentityType")}
          </dt>
          <dd className="flex min-h-7 items-center">{identityType}</dd>
        </div>
      ) : null}
      {agent && agentStatus ? (
        <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
          <dt className="flex min-h-7 items-center text-xs text-muted-foreground">
            {t("contextAgentStatus")}
          </dt>
          <dd className="flex min-h-7 items-center">{agentStatus}</dd>
        </div>
      ) : null}
    </dl>
  )
}

/** 展示当前会话摘要和类型对应的资料内容。 */
function ConversationContextContent({
  conversation,
  directTarget,
  displayName,
  currentIdentityID,
  replyDisabledReason,
  customerDraftRef,
  onGroupLeft,
  onClose,
}: {
  conversation: InboxConversation | null
  directTarget: MemberOption | null
  displayName: string
  currentIdentityID: string
  replyDisabledReason: string | null
  customerDraftRef: RefObject<ComposerDraftBridge | null>
  onGroupLeft: () => void
  onClose: () => void
}) {
  const { t } = useTranslation("inbox")
  const [customerTab, setCustomerTab] = useState("profile")
  const customer =
    conversation && isCustomerInboxConversation(conversation)
      ? conversation.customer
      : null
  const group =
    conversation && isGroupInboxConversation(conversation)
      ? conversation.group
      : null
  return (
    <div className="relative flex h-full min-h-0 min-w-0 flex-col overflow-x-visible overflow-y-hidden bg-background">
      {/* 页签行右端常驻收起入口，三类上下文共用同一位置。 */}
      <HeaderAction
        label={t("contextClose")}
        icon={PanelRightCloseIcon}
        className="absolute top-1.5 right-2 z-10"
        onClick={onClose}
      />
      {conversation && customer ? (
        <Tabs
          key={conversation.id}
          value={customerTab}
          onValueChange={setCustomerTab}
          className="min-h-0 flex-1"
        >
          <TabsList
            aria-label={t("contextTabsLabel")}
            className="h-auto shrink-0 justify-start gap-1 px-3 py-2"
          >
            <TabsTrigger
              value="profile"
              className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
            >
              {t("contextProfileTab")}
            </TabsTrigger>
            <TabsTrigger
              value="assistant"
              className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
            >
              {t("contextAssistantTab")}
            </TabsTrigger>
            <TabsTrigger
              value="business"
              className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
            >
              {t("contextBusinessTab")}
            </TabsTrigger>
          </TabsList>

          <TabsContent
            value="profile"
            className="mt-0 min-h-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-contain p-3"
          >
            <section className="space-y-2">
              <dl className="space-y-1 text-sm">
                <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
                  <dt className="flex min-h-7 min-w-0 items-center text-xs text-muted-foreground">
                    {t("contextContactName")}
                  </dt>
                  <dd className="flex min-h-7 min-w-0 items-center gap-2">
                    <ConversationAvatar
                      conversation={conversation}
                      className="size-7 text-xs"
                    />
                    <span className="min-w-0 truncate" title={displayName}>
                      {displayName}
                    </span>
                  </dd>
                </div>
              </dl>
              <p className="text-xs leading-5 text-muted-foreground">
                {t("contextContactDetailsPlaceholder")}
              </p>
            </section>
          </TabsContent>

          <TabsContent
            value="assistant"
            forceMount
            className="mt-0 flex min-h-0 flex-1 flex-col overflow-hidden data-[state=inactive]:hidden"
          >
            <CustomerCopilotPanel
              key={conversation.id}
              customerConversationID={conversation.id}
              replyDisabledReason={replyDisabledReason}
              customerDraftRef={customerDraftRef}
              active={customerTab === "assistant"}
            />
          </TabsContent>

          <TabsContent
            value="business"
            className="mt-0 min-h-0 flex-1 overflow-hidden"
          >
            <ContextPlaceholder
              icon={BriefcaseBusinessIcon}
              title={t("contextBusinessTitle")}
              description={t("contextBusinessDescription")}
            />
          </TabsContent>
        </Tabs>
      ) : conversation && group ? (
        <GroupConversationContext
          conversationID={conversation.id}
          currentIdentityID={currentIdentityID}
          onLeft={onGroupLeft}
        />
      ) : (
        <Tabs
          key={conversation?.id ?? directTarget?.id}
          defaultValue="profile"
          className="min-h-0 flex-1"
        >
          <TabsList
            aria-label={t("contextTabsLabel")}
            className="h-auto shrink-0 justify-start gap-1 px-3 py-2"
          >
            <TabsTrigger
              value="profile"
              className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
            >
              {t("contextProfileTab")}
            </TabsTrigger>
          </TabsList>
          <TabsContent
            value="profile"
            className="mt-0 min-h-0 flex-1 overflow-y-auto overscroll-contain p-3"
          >
            <InternalConversationProfile
              conversation={conversation}
              directTarget={directTarget}
              displayName={displayName}
            />
          </TabsContent>
        </Tabs>
      )}
    </div>
  )
}

/** 展示可调整宽度和收起状态的会话资料栏。 */
export function ConversationContextPane({
  conversation,
  directTarget,
  displayName,
  currentIdentityID,
  replyDisabledReason,
  customerDraftRef,
  onGroupLeft,
  visible,
  onClose,
}: {
  conversation: InboxConversation | null
  directTarget: MemberOption | null
  displayName: string
  currentIdentityID: string
  replyDisabledReason: string | null
  customerDraftRef: RefObject<ComposerDraftBridge | null>
  onGroupLeft: () => void
  visible: boolean
  onClose: () => void
}) {
  const { t } = useTranslation("inbox")
  const trackRef = useRef<HTMLDivElement>(null)
  const [rowWidth, setRowWidth] = useState(0)
  // 展开宽度默认取默认档位，剩余空间不足时按会话区的保留宽度收窄。
  const [desiredWidth, setDesiredWidth] = useState(contextPanelDefaultWidth)
  const maxWidth = rowWidth
    ? Math.min(contextPanelMaxWidth, Math.max(Math.round(rowWidth / 2), rowWidth - conversationMinWidth))
    : contextPanelMaxWidth
  const contextPanelWidth = Math.min(desiredWidth, maxWidth)

  useEffect(() => {
    const row = trackRef.current?.parentElement
    if (!row) return
    const observer = new ResizeObserver(([entry]) => setRowWidth(entry.contentRect.width))
    observer.observe(row)
    return () => observer.disconnect()
  }, [])
  /** 结束拖动联系人上下文栏。 */
  function stopContextPanelResize(event: ReactPointerEvent<HTMLButtonElement>) {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
  }

  return (
    <>
      <div ref={trackRef} className="relative h-full min-h-0 w-0 shrink-0">
        {visible ? (
          <button
            type="button"
            className="absolute top-0 left-0 z-20 h-full w-2 -translate-x-1 cursor-col-resize touch-none"
            aria-label={t("contextResize")}
            onPointerDown={(event) => {
              // 开始拖动联系人上下文栏。
              event.preventDefault()
              event.currentTarget.setPointerCapture(event.pointerId)
            }}
            onPointerMove={(event) => {
              if (!event.currentTarget.hasPointerCapture(event.pointerId)) {
                return
              }
              // 按指针位置调整宽度，下限为最小宽度与当前允许最大值中较小的一个。
              const width = Math.max(
                Math.min(contextPanelMinWidth, maxWidth),
                window.innerWidth - event.clientX,
              )
              setDesiredWidth(Math.min(maxWidth, width))
            }}
            onPointerUp={stopContextPanelResize}
            onPointerCancel={stopContextPanelResize}
          />
        ) : null}
      </div>

      <aside
        className={cn(
          "cervi-conversation-context-pane relative h-full min-h-0 min-w-0 shrink-0 overflow-hidden border-l bg-background",
          !visible && "hidden",
        )}
        style={{ width: contextPanelWidth }}
      >
        <ConversationContextContent
          conversation={conversation}
          directTarget={directTarget}
          displayName={displayName}
          currentIdentityID={currentIdentityID}
          replyDisabledReason={replyDisabledReason}
          customerDraftRef={customerDraftRef}
          onGroupLeft={onGroupLeft}
          onClose={onClose}
        />
      </aside>
    </>
  )
}
