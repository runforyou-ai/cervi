/** 客服、单聊与群聊共用的会话资料栏。 */
import {
  BotIcon,
  BriefcaseBusinessIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  GlobeIcon,
  MessageCircleIcon,
  MessagesSquareIcon,
  SendIcon,
  UserRoundIcon,
  UsersRoundIcon,
} from "lucide-react"
import { useState, type PointerEvent as ReactPointerEvent } from "react"
import { useTranslation } from "react-i18next"

import {
  ChannelType,
  OrganizationIdentityType,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type InboxConversation,
} from "@/api"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"
import { ConversationAvatar } from "@/features/inbox/conversation-header"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import { GroupParticipantList } from "@/features/inbox/group-participant-list"
import { cn } from "@/lib/utils"

const contextPanelMinWidth = 320
const contextPanelMaxWidth = 640
const contextPanelToggleWidth = 16

const channelIcons: Partial<Record<ChannelType, typeof GlobeIcon>> = {
  [ChannelType.ChannelTypeWebsite]: GlobeIcon,
  [ChannelType.ChannelTypeTelegram]: SendIcon,
  [ChannelType.ChannelTypeWeChatOfficialAccount]: MessageCircleIcon,
}

/** 把联系人上下文栏宽度限制在桌面可用范围内。 */
function clampContextPanelWidth(width: number) {
  return Math.min(
    contextPanelMaxWidth,
    Math.max(contextPanelMinWidth, width),
  )
}

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

/** 展示单聊或群聊的基础资料。 */
function InternalConversationProfile({
  conversation,
  displayName,
}: {
  conversation: InboxConversation
  displayName: string
}) {
  const { t } = useTranslation("inbox")
  const direct = isDirectInboxConversation(conversation)
    ? conversation.direct
    : null
  const group = isGroupInboxConversation(conversation)
    ? conversation.group
    : null
  const identityType =
    direct?.peerType ===
    OrganizationIdentityType.OrganizationIdentityTypeAgent
      ? t("contextIdentityAgent")
      : t("contextIdentityMember")
  const agentStatus = agentRunStatusLabel(direct?.agentRunStatus ?? null, t)

  return (
    <dl className="space-y-2 text-sm">
      <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
        <dt className="flex min-h-8 items-center text-xs text-muted-foreground">
          {t("contextContactName")}
        </dt>
        <dd className="flex min-h-8 min-w-0 items-center gap-2">
          <ConversationAvatar
            conversation={conversation}
            className="size-7 rounded-full text-xs"
          />
          <span className="min-w-0 truncate" title={displayName}>
            {displayName}
          </span>
        </dd>
      </div>
      {direct ? (
        <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
          <dt className="flex min-h-8 items-center text-xs text-muted-foreground">
            {t("contextIdentityType")}
          </dt>
          <dd className="flex min-h-8 items-center">{identityType}</dd>
        </div>
      ) : null}
      {direct && agentStatus ? (
        <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
          <dt className="flex min-h-8 items-center text-xs text-muted-foreground">
            {t("contextAgentStatus")}
          </dt>
          <dd className="flex min-h-8 items-center">{agentStatus}</dd>
        </div>
      ) : null}
      {group ? (
        <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
          <dt className="flex min-h-8 items-center text-xs text-muted-foreground">
            {t("contextGroupMemberCount")}
          </dt>
          <dd className="flex min-h-8 items-center">
            {t("groupMemberCount", { count: group.memberCount })}
          </dd>
        </div>
      ) : null}
    </dl>
  )
}

/** 展示当前会话摘要和类型对应的资料内容。 */
function ConversationContextContent({
  conversation,
  displayName,
  sessionStatus,
  sheet = false,
}: {
  conversation: InboxConversation
  displayName: string
  sessionStatus: string
  sheet?: boolean
}) {
  const { t } = useTranslation("inbox")
  const customer = isCustomerInboxConversation(conversation)
    ? conversation.customer
    : null
  const direct = isDirectInboxConversation(conversation)
    ? conversation.direct
    : null
  const group = isGroupInboxConversation(conversation)
    ? conversation.group
    : null
  const HeaderIcon = customer
    ? (channelIcons[customer.channelType] ?? MessagesSquareIcon)
    : group
      ? UsersRoundIcon
      : direct?.peerType ===
          OrganizationIdentityType.OrganizationIdentityTypeAgent
        ? BotIcon
        : UserRoundIcon
  const headerTitle = customer?.channelName ?? displayName

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-col overflow-x-visible overflow-y-hidden bg-background">
      <div
        className={cn(
          "shrink-0 space-y-1.5 border-b px-3 py-2.5",
          sheet && "pr-12",
        )}
      >
        <div className="flex items-center gap-2">
          <HeaderIcon className="size-4 shrink-0 text-muted-foreground" />
          <span
            className="min-w-0 truncate text-sm font-medium text-foreground"
            title={headerTitle}
          >
            {headerTitle}
          </span>
        </div>
        {customer ? (
          <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>
              {t("contextReceptionStatus")}：
              <span className="text-foreground">{sessionStatus}</span>
            </span>
            <span className="min-w-0 truncate" title={customer.title}>
              {t("contextConversationLabel")}：
              <span className="text-foreground">{customer.title}</span>
            </span>
          </div>
        ) : group ? (
          <p className="text-xs text-muted-foreground">
            {t("groupMemberCount", { count: group.memberCount })}
          </p>
        ) : (
          <p className="text-xs text-muted-foreground">
            {direct?.peerType ===
            OrganizationIdentityType.OrganizationIdentityTypeAgent
              ? t("contextIdentityAgent")
              : t("contextIdentityMember")}
          </p>
        )}
      </div>

      {customer ? (
        <Tabs
          key={conversation.id}
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
            <section className="space-y-4">
              <dl className="space-y-2 text-sm">
                <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
                  <dt className="flex min-h-8 min-w-0 items-center text-xs text-muted-foreground">
                    {t("contextContactName")}
                  </dt>
                  <dd className="flex min-h-8 min-w-0 items-center gap-2">
                    <ConversationAvatar
                      conversation={conversation}
                      className="size-7 rounded-full text-xs"
                    />
                    <span className="min-w-0 truncate" title={displayName}>
                      {displayName}
                    </span>
                  </dd>
                </div>
              </dl>
              <Separator />
              <p className="text-xs leading-5 text-muted-foreground">
                {t("contextContactDetailsPlaceholder")}
              </p>
            </section>
          </TabsContent>

          <TabsContent
            value="assistant"
            className="mt-0 min-h-0 flex-1 overflow-hidden"
          >
            <ContextPlaceholder
              icon={BotIcon}
              title={t("contextAssistantTitle")}
              description={t("contextAssistantDescription")}
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
      ) : group ? (
        <Tabs
          key={conversation.id}
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
              {t("contextGroupProfileTab")}
            </TabsTrigger>
            <TabsTrigger
              value="members"
              className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
            >
              {t("contextGroupMembersTab")}
            </TabsTrigger>
          </TabsList>
          <TabsContent
            value="profile"
            className="mt-0 min-h-0 flex-1 overflow-y-auto overscroll-contain p-3"
          >
            <InternalConversationProfile
              conversation={conversation}
              displayName={displayName}
            />
          </TabsContent>
          <TabsContent
            value="members"
            className="mt-0 min-h-0 flex-1 overflow-y-auto overscroll-contain p-1.5"
          >
            <GroupParticipantList conversationID={conversation.id} />
          </TabsContent>
        </Tabs>
      ) : (
        <Tabs
          key={conversation.id}
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
              displayName={displayName}
            />
          </TabsContent>
        </Tabs>
      )}
    </div>
  )
}

/** 在宽屏常驻栏和较窄视口 Sheet 中复用联系人上下文。 */
export function ConversationContextPane({
  conversation,
  displayName,
  sessionStatus,
  title,
  description,
  desktopVisible,
  sheetOpen,
  onDesktopToggle,
  onSheetOpenChange,
}: {
  conversation: InboxConversation
  displayName: string
  sessionStatus: string
  title: string
  description: string
  desktopVisible: boolean
  sheetOpen: boolean
  onDesktopToggle: () => void
  onSheetOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation("inbox")
  const [contextPanelWidth, setContextPanelWidth] = useState(380)

  /** 开始拖动联系人上下文栏。 */
  function startContextPanelResize(
    event: ReactPointerEvent<HTMLButtonElement>,
  ) {
    event.preventDefault()
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  /** 按指针位置调整联系人上下文栏宽度。 */
  function resizeContextPanel(event: ReactPointerEvent<HTMLButtonElement>) {
    if (!event.currentTarget.hasPointerCapture(event.pointerId)) return
    setContextPanelWidth(
      clampContextPanelWidth(
        window.innerWidth - event.clientX - contextPanelToggleWidth,
      ),
    )
  }

  /** 结束拖动联系人上下文栏。 */
  function stopContextPanelResize(
    event: ReactPointerEvent<HTMLButtonElement>,
  ) {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
  }

  return (
    <>
      <div
        className={cn(
          "relative hidden h-full min-h-0 w-4 shrink-0 bg-background xl:block",
          desktopVisible && "border-l",
        )}
      >
        {desktopVisible ? (
          <button
            type="button"
            className="absolute top-0 left-0 z-20 h-full w-2 -translate-x-1 cursor-col-resize touch-none"
            aria-label={t("contextResize")}
            onPointerDown={startContextPanelResize}
            onPointerMove={resizeContextPanel}
            onPointerUp={stopContextPanelResize}
            onPointerCancel={stopContextPanelResize}
          />
        ) : null}
        <button
          type="button"
          className={cn(
            "absolute top-1/2 left-0 z-30 flex h-12 w-4 -translate-y-1/2 items-center justify-center border border-border bg-muted text-muted-foreground shadow-sm transition-colors hover:bg-muted/80 hover:text-foreground",
            desktopVisible
              ? "rounded-r-md border-l-0"
              : "rounded-l-md border-r-0",
          )}
          aria-label={
            desktopVisible ? t("contextClose") : t("contextOpen")
          }
          title={desktopVisible ? t("contextClose") : t("contextOpen")}
          onClick={onDesktopToggle}
        >
          {desktopVisible ? (
            <ChevronRightIcon className="size-3" />
          ) : (
            <ChevronLeftIcon className="size-3" />
          )}
        </button>
      </div>

      <aside
        className={cn(
          "relative hidden h-full min-h-0 min-w-0 shrink-0 overflow-hidden bg-background xl:block",
          !desktopVisible && "xl:hidden",
        )}
        style={{ width: contextPanelWidth }}
      >
        <ConversationContextContent
          conversation={conversation}
          displayName={displayName}
          sessionStatus={sessionStatus}
        />
      </aside>

      <Sheet open={sheetOpen} onOpenChange={onSheetOpenChange}>
        <SheetContent className="data-[side=right]:w-full gap-0 p-0 sm:max-w-sm">
          <SheetHeader className="sr-only">
            <SheetTitle>{title}</SheetTitle>
            <SheetDescription>{description}</SheetDescription>
          </SheetHeader>
          <ConversationContextContent
            conversation={conversation}
            displayName={displayName}
            sessionStatus={sessionStatus}
            sheet
          />
        </SheetContent>
      </Sheet>
    </>
  )
}
