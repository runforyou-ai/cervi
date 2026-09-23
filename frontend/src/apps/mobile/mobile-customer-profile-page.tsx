/** 移动端客户会话的客户资料子页：客户名称、来源渠道、身份与访客上下文。 */
import { useTranslation } from "react-i18next"
import { useOutletContext } from "react-router"

import { ChannelType } from "@/api"
import type { MobileCustomerConversationContext } from "@/apps/mobile/mobile-customer-conversation-page"
import {
  MobilePageHeader,
  MobileProfileField,
  MobileScrollArea,
} from "@/apps/mobile/mobile-page"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import { CustomerProfileDetails } from "@/features/inbox/customer-profile-details"
import { useConversationName } from "@/features/inbox/use-conversation-name"

/** 全屏展示客户资料，返回时回到客户会话。 */
export function MobileCustomerProfilePage() {
  const { t } = useTranslation("inbox")
  const { conversation } = useOutletContext<MobileCustomerConversationContext>()
  const conversationName = useConversationName()
  const { customer } = conversation

  return (
    <section className="absolute inset-0 flex min-h-0 flex-col bg-background">
      <MobilePageHeader
        backTo={`/inbox/customer/${conversation.id}`}
        title={t("customerProfile")}
      />
      <MobileScrollArea
        storageKey={`customer-profile:${conversation.id}`}
        className="px-4 py-6"
      >
        <div className="flex items-center gap-3 pb-6">
          <ConversationAvatar conversation={conversation} className="size-14 text-xl" />
          <div className="min-w-0">
            <h2 className="break-words text-lg font-semibold">
              {conversationName(conversation)}
            </h2>
            <p className="truncate text-sm text-muted-foreground">
              {customer.channelName}
            </p>
          </div>
        </div>
        <dl className="divide-y border-y empty:hidden">
          <CustomerProfileDetails
            conversationID={conversation.id}
            lastMessageID={conversation.lastMessageId}
            website={customer.channelType === ChannelType.ChannelTypeWebsite}
            field={MobileProfileField}
          />
        </dl>
      </MobileScrollArea>
    </section>
  )
}
