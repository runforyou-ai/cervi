/** 展示尚未创建会话的单聊目标。 */
import { MoreVerticalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { OrganizationIdentityType, type MemberOption } from "@/api"
import { ProfileAvatar } from "@/components/profile-avatar"
import { HeaderAction } from "@/features/inbox/conversation-header"

/** 展示草稿收件人的头像。 */
export function DirectConversationDraftAvatar({
  member,
  className = "size-8",
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
export function DirectConversationDraftHeader({
  member,
  contextVisible = false,
  onToggleContext,
}: {
  member: MemberOption
  contextVisible?: boolean
  onToggleContext?: () => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <header className="flex shrink-0 items-center gap-2.5 px-3 py-2">
      <DirectConversationDraftAvatar member={member} />
      <h2 className="min-w-0 flex-1 truncate text-sm font-semibold">{member.displayName}</h2>
      {onToggleContext ? (
        <HeaderAction
          label={contextVisible ? t("contextClose") : t("contextOpen")}
          icon={MoreVerticalIcon}
          onClick={onToggleContext}
        />
      ) : null}
    </header>
  )
}
