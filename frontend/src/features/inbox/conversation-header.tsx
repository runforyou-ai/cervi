/** 客户会话头与联系人头像。 */
import { useEffect, useState } from "react"
import {
  GlobeIcon,
  MessageCircleIcon,
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
  const [avatarFailed, setAvatarFailed] = useState(false)

  useEffect(() => setAvatarFailed(false), [conversation.contactAvatarUrl])

  return (
    <div className="relative shrink-0">
      <div
        className={cn(
          "flex size-10 items-center justify-center overflow-hidden rounded-lg bg-muted text-sm font-medium text-muted-foreground",
          className,
        )}
      >
        {conversation.contactAvatarUrl && !avatarFailed ? (
          <img
            src={conversation.contactAvatarUrl}
            alt=""
            className="size-full rounded-[inherit] object-cover"
            draggable={false}
            onError={() => setAvatarFailed(true)}
          />
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
        <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
          <span className="inline-flex h-5 shrink-0 items-center rounded-md border px-1.5 text-[10px]">
            {sessionStatus}
          </span>
          <span
            className="inline-flex h-5 min-w-0 items-center truncate rounded-md border px-1.5 text-[10px]"
            title={conversation.title}
          >
            {conversation.title}
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
          {t("conversationTransfer")}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="hidden lg:inline-flex"
          disabled
        >
          {t("conversationHandToAi")}
        </Button>
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
    </header>
  )
}
