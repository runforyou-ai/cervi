/** 客服、单聊与群聊共用的会话资料栏。 */
import {
  BotIcon,
  BriefcaseBusinessIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
} from "lucide-react"
import {
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react"
import { useTranslation } from "react-i18next"

import {
  OrganizationIdentityType,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type GroupConversationData,
  type InboxConversation,
  type MemberOption,
} from "@/api"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"
import { ConversationAvatar } from "@/features/inbox/conversation-header"
import { DirectConversationDraftAvatar } from "@/features/inbox/direct-conversation-draft-header"
import { agentRunStatusLabel } from "@/features/inbox/agent-run-status"
import { GroupConversationContext } from "@/features/inbox/group-conversation-context"
import { cn } from "@/lib/utils"

const contextPanelMinWidth = 320
const contextPanelMaxWidth = 640
const contextPanelToggleWidth = 16

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
  const identityType =
    (direct?.peerType ?? directTarget?.type) ===
    OrganizationIdentityType.OrganizationIdentityTypeAgent
      ? t("contextIdentityAgent")
      : t("contextIdentityMember")
  const agentStatus = agentRunStatusLabel(direct?.agentRunStatus ?? null, t)

  return (
    <dl className="space-y-1 text-sm">
      <div className="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-2">
        <dt className="flex min-h-8 items-center text-xs text-muted-foreground">
          {t("contextContactName")}
        </dt>
        <dd className="flex min-h-8 min-w-0 items-center gap-2">
          {conversation ? (
            <ConversationAvatar
              conversation={conversation}
              className="size-7 rounded-full text-xs"
            />
          ) : directTarget ? (
            <DirectConversationDraftAvatar member={directTarget} className="size-7 text-xs" />
          ) : null}
          <span className="min-w-0 truncate" title={displayName}>
            {displayName}
          </span>
        </dd>
      </div>
      {direct || directTarget ? (
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
    </dl>
  )
}

/** 展示当前会话摘要和类型对应的资料内容。 */
function ConversationContextContent({
  conversation,
  directTarget,
  displayName,
  currentIdentityID,
  groupDraft,
  onGroupDraftChange,
  onGroupSummaryChange,
  onGroupLeft,
}: {
  conversation: InboxConversation | null
  directTarget: MemberOption | null
  displayName: string
  currentIdentityID: string
  groupDraft: GroupConversationData | null
  onGroupDraftChange: (group: GroupConversationData) => void
  onGroupSummaryChange: (changes: {
    title?: string
    imageUrl?: string
    memberCount?: number
    status?: GroupConversationData["status"]
  }) => void
  onGroupLeft: () => void
}) {
  const { t } = useTranslation("inbox")
  const customer =
    conversation && isCustomerInboxConversation(conversation) ? conversation.customer : null
  const group =
    conversation && isGroupInboxConversation(conversation) ? conversation.group : null
  return (
    <div className="flex h-full min-h-0 min-w-0 flex-col overflow-x-visible overflow-y-hidden bg-background">
      {conversation && customer ? (
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
            <section className="space-y-2">
              <dl className="space-y-1 text-sm">
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
      ) : conversation && group ? (
        <GroupConversationContext
          conversationID={conversation.id}
          currentIdentityID={currentIdentityID}
          draft={groupDraft}
          onDraftChange={onGroupDraftChange}
          onSummaryChange={onGroupSummaryChange}
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
  onGroupSummaryChange,
  onGroupLeft,
  visible,
  onToggle,
}: {
  conversation: InboxConversation | null
  directTarget: MemberOption | null
  displayName: string
  currentIdentityID: string
  onGroupSummaryChange: (changes: {
    title?: string
    imageUrl?: string
    memberCount?: number
    status?: GroupConversationData["status"]
  }) => void
  onGroupLeft: () => void
  visible: boolean
  onToggle: () => void
}) {
  const { t } = useTranslation("inbox")
  const [contextPanelWidth, setContextPanelWidth] = useState(contextPanelMinWidth)
  const [groupDraft, setGroupDraft] = useState<{
    conversationID: string
    group: GroupConversationData
  } | null>(null)
  const activeGroupDraft =
    groupDraft && groupDraft.conversationID === conversation?.id
      ? groupDraft.group
      : null

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
          "relative h-full min-h-0 w-4 shrink-0 bg-background",
          visible && "border-l",
        )}
      >
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
              // 按指针位置调整并限制联系人上下文栏宽度。
              const width = Math.max(
                contextPanelMinWidth,
                window.innerWidth - event.clientX - contextPanelToggleWidth,
              )
              setContextPanelWidth(Math.min(contextPanelMaxWidth, width))
            }}
            onPointerUp={stopContextPanelResize}
            onPointerCancel={stopContextPanelResize}
          />
        ) : null}
        <button
          type="button"
          className={cn(
            "absolute top-1/2 left-0 z-30 flex h-12 w-4 -translate-y-1/2 items-center justify-center border border-border bg-muted text-muted-foreground shadow-sm transition-colors hover:bg-muted/80 hover:text-foreground",
            visible
              ? "rounded-r-md border-l-0"
              : "rounded-l-md border-r-0",
          )}
          aria-label={visible ? t("contextClose") : t("contextOpen")}
          title={visible ? t("contextClose") : t("contextOpen")}
          onClick={onToggle}
        >
          {visible ? (
            <ChevronRightIcon className="size-3" />
          ) : (
            <ChevronLeftIcon className="size-3" />
          )}
        </button>
      </div>

      <aside
        className={cn(
          "cervi-conversation-context-pane relative h-full min-h-0 min-w-0 shrink-0 overflow-hidden bg-background",
          !visible && "hidden",
        )}
        style={{ width: contextPanelWidth }}
      >
        <ConversationContextContent
          conversation={conversation}
          directTarget={directTarget}
          displayName={displayName}
          currentIdentityID={currentIdentityID}
          groupDraft={activeGroupDraft}
          onGroupDraftChange={(group) =>
            setGroupDraft({ conversationID: group.id, group })
          }
          onGroupSummaryChange={onGroupSummaryChange}
          onGroupLeft={onGroupLeft}
        />
      </aside>
    </>
  )
}
