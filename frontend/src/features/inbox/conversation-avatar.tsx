/** 会话列表和会话头共用的头像与渠道角标。 */
import {
  isCustomerInboxConversation,
  isAgentInboxConversation,
  isDirectInboxConversation,
  isGroupInboxConversation,
  type InboxConversation,
} from "@/api"
import { ProfileAvatar } from "@/components/profile-avatar"
import { messageChannelTypeDefinition } from "@/lib/message-channel-types"
import { cn } from "@/lib/utils"

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
  const agent = isAgentInboxConversation(conversation) ? conversation.agent : null
  const badge = customer
    ? messageChannelTypeDefinition(customer.channelType)
    : undefined
  const contactName =
    customer?.contactName?.trim() ||
    direct?.peerName.trim() || agent?.agentName.trim() ||
    group?.title.trim()
  const avatarURL =
    customer?.contactAvatarUrl ?? direct?.peerAvatarUrl ?? agent?.agentAvatarUrl ?? group?.imageUrl
  const fallback = group
    ? "group"
    : agent
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
            "absolute -right-0.5 -bottom-0.5 flex size-3.5 items-center justify-center rounded-full border-2 border-background text-white",
            badge.badgeClassName,
          )}
        >
          <badge.icon className="size-2" />
        </span>
      ) : null}
    </div>
  )
}
