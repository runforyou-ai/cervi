/** 展示尚未创建会话的单聊目标。 */
import { PanelRightOpenIcon } from "lucide-react"
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { OrganizationIdentityType, type MemberOption } from "@/api"
import { ProfileAvatar } from "@/components/profile-avatar"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
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
  actions,
  contextVisible = false,
  onToggleContext,
}: {
  member: MemberOption
  /** 展示在侧栏开关之前的草稿操作。 */
  actions?: ReactNode
  contextVisible?: boolean
  onToggleContext?: () => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <header
      data-slot="conversation-header"
      className="flex min-h-12 shrink-0 items-center gap-2.5 px-3 py-2"
    >
      {/* 标题只占文字宽度，右侧留白保持窗口可拖动。 */}
      <div className="flex min-w-0 flex-1 items-center">
        <Tooltip>
          <TooltipTrigger asChild>
            <h2
              data-slot="conversation-header-title"
              className="min-w-0 truncate text-xl font-semibold"
            >
              {member.displayName}
            </h2>
          </TooltipTrigger>
          <TooltipContent className="max-w-80">{member.displayName}</TooltipContent>
        </Tooltip>
      </div>
      {actions}
      {onToggleContext && !contextVisible ? (
        <HeaderAction
          label={t("sidePanelOpen")}
          icon={PanelRightOpenIcon}
          onClick={onToggleContext}
        />
      ) : null}
    </header>
  )
}
