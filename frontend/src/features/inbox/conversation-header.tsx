import {
  ArrowRightLeftIcon,
  BotIcon,
  GlobeIcon,
  MessageCircleIcon,
  MessagesSquareIcon,
  PanelRightIcon,
  SendIcon,
  UserRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import { ChannelType, type InboxConversation } from "@/api"
import { Button } from "@/components/ui/button"
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
  const badge = sourceBadges[conversation.channelType]
  const contactName = conversation.contactName?.trim()

  return (
    <div className="relative shrink-0">
      <div
        className={cn(
          "flex size-10 items-center justify-center rounded-lg bg-muted text-sm font-medium text-muted-foreground",
          className,
        )}
      >
        {contactName ? (
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

/** 展示当前 Conversation 标题和已接入的处理摘要。 */
export function ConversationHeader({
  conversation,
  sessionStatus,
  contextVisible,
  narrowViewport = false,
  onContextToggle,
}: {
  conversation: InboxConversation
  sessionStatus: string
  contextVisible: boolean
  narrowViewport?: boolean
  onContextToggle: () => void
}) {
  const { t } = useTranslation("inbox")
  const contextActionLabel = contextVisible
    ? t("contextClose")
    : t("contextOpen")

  return (
    <header
      data-slot="conversation-header"
      className={cn(
        "flex h-16 shrink-0 items-center gap-3 border-b border-border/60 px-4",
        narrowViewport && "pr-14",
      )}
    >
      <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border bg-muted/30 text-muted-foreground">
        <MessagesSquareIcon className="size-4" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
            {t("conversationCurrent")}
          </span>
          <span className="truncate rounded-full bg-muted px-2 py-0.5 text-[10px] text-muted-foreground">
            {sessionStatus}
          </span>
        </div>
        <div className="mt-0.5 flex min-w-0 items-baseline gap-2">
          <h2 className="truncate text-sm font-semibold">
            {conversation.title}
          </h2>
          <span className="shrink-0 text-xs text-muted-foreground">
            {conversation.channelName}
          </span>
        </div>
      </div>
      <div
        data-slot="conversation-actions"
        className="flex shrink-0 items-center gap-2"
      >
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="hidden lg:inline-flex"
          disabled
        >
          <ArrowRightLeftIcon />
          {t("conversationTransfer")}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="hidden lg:inline-flex"
          disabled
        >
          <BotIcon />
          {t("conversationHandToAi")}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="icon-sm"
          className="xl:hidden"
          aria-label={contextActionLabel}
          aria-pressed={contextVisible}
          title={contextActionLabel}
          onClick={onContextToggle}
        >
          <PanelRightIcon />
        </Button>
      </div>
    </header>
  )
}
