/** 会话列表和会话头共用的头像与渠道角标。 */
import { GlobeIcon, MessageCircleIcon, SendIcon } from "lucide-react"

import {
  ChannelType,
  OrganizationIdentityType,
  isCustomerInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type InboxConversation,
} from "@/api"
import { ProfileAvatar } from "@/components/profile-avatar"
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

/** 展示会话对象头像和客户来源渠道角标。 */
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
  const group = isGroupInboxConversation(conversation)
    ? conversation.group
    : null
  const badge = customer ? sourceBadges[customer.channelType] : undefined
  const contactName =
    customer?.contactName?.trim() ||
    direct?.peerName.trim() ||
    group?.title.trim()
  const avatarURL =
    customer?.contactAvatarUrl ?? direct?.peerAvatarUrl ?? group?.imageUrl
  const fallback = group
    ? "group"
    : direct?.peerType === OrganizationIdentityType.OrganizationIdentityTypeAgent
      ? "agent"
      : "person"

  return (
    <div className="relative shrink-0">
      <ProfileAvatar
        imageURL={avatarURL}
        name={contactName}
        fallback={fallback}
        className={className}
      />
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
