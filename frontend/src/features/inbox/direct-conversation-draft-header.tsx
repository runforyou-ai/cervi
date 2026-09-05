/** 展示尚未创建会话的单聊目标。 */
import { BotIcon, UserRoundIcon } from "lucide-react"
import { OrganizationIdentityType, type MemberOption } from "@/api"
import { cn } from "@/lib/utils"

/** 展示草稿收件人的头像。 */
export function DirectConversationDraftAvatar({
  member,
  className,
}: {
  member: MemberOption
  className?: string
}) {
  return (
    <span className={cn(
      "flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-medium text-primary",
      className,
    )}>
      {member.type === OrganizationIdentityType.OrganizationIdentityTypeAgent ? (
        <BotIcon className="size-4.5" />
      ) : member.displayName ? (
        Array.from(member.displayName)[0]
      ) : (
        <UserRoundIcon className="size-4.5" />
      )}
    </span>
  )
}

/** 展示草稿收件人的姓名和身份。 */
export function DirectConversationDraftHeader({ member }: { member: MemberOption }) {
  return (
    <header className="flex shrink-0 items-center gap-3 border-b px-4 py-3">
      <DirectConversationDraftAvatar member={member} />
      <h2 className="min-w-0 truncate text-sm font-semibold">{member.displayName}</h2>
    </header>
  )
}
