/** 展示尚未创建会话的单聊目标。 */
import { OrganizationIdentityType, type MemberOption } from "@/api"
import { ProfileAvatar } from "@/components/profile-avatar"

/** 展示草稿收件人的头像。 */
export function DirectConversationDraftAvatar({
  member,
  className = "size-9",
}: {
  member: MemberOption
  className?: string
}) {
  return (
    <ProfileAvatar
      imageURL={member.avatarUrl}
      name={member.displayName}
      fallback={
        member.type === OrganizationIdentityType.OrganizationIdentityTypeAgent
          ? "agent"
          : "person"
      }
      className={className}
    />
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
