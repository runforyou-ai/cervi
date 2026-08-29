/** 展示客户 Conversation 标题、来源摘要和上下文栏入口。 */
import {
  GlobeIcon,
  MessageCircleIcon,
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

  return (
    <div className="relative shrink-0">
      <div
        className={cn(
          "flex size-10 items-center justify-center rounded-lg bg-muted text-sm font-medium text-muted-foreground",
          className,
        )}
      >
        {conversation.contactName ? (
          conversation.contactName.slice(0, 1).toLocaleUpperCase()
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
  contactName,
  sessionStatus,
  contextVisible,
  narrowViewport = false,
  onContextToggle,
}: {
  conversation: InboxConversation
  contactName: string
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
      <ConversationAvatar conversation={conversation} />
      <div className="min-w-0 flex-1">
        <h2 className="truncate text-sm font-semibold">{conversation.title}</h2>
        <p className="mt-0.5 truncate text-xs text-muted-foreground">
          {contactName} · {conversation.channelName} · {sessionStatus}
        </p>
      </div>
      <div
        data-slot="conversation-actions"
        className="flex shrink-0 items-center"
      >
        <Button
          type="button"
          variant="outline"
          size="icon-sm"
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
