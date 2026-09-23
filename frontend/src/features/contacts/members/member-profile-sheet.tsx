/** 同事只读资料侧滑面板，供同事列表和团队成员列表共用。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { UserStatus, getUser, isApiError, sessionPath } from "@/api"
import { ReadonlyDetailRow } from "@/components/form/detail-edit-row"
import { ProfileAvatar } from "@/components/profile-avatar"
import { Button } from "@/components/ui/button"
import { WorkStatusBadge } from "@/components/work-status"
import { useWorkspace } from "@/contexts/workspace-context"
import { ContactDetailSheet } from "@/features/contacts/contact-detail-sheet"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 按用户编号加载并展示同事资料，读取失败时关闭；可向其他在职同事发消息。 */
export function MemberProfileSheet({
  userId,
  onClose,
}: {
  userId: string
  onClose: () => void
}) {
  const { t } = useTranslation("contacts")
  const { identity } = useWorkspace()
  const navigate = useNavigate()
  const detail = useResource(resourceKeys.user(userId), () => getUser(userId), {
    enabled: Boolean(userId),
  })
  const user = userId ? (detail.data ?? null) : null
  // 自己的工作状态以当前身份为准。
  const workStatus =
    user?.id === identity.user.id ? identity.user.workStatus : user?.workStatus

  const detailError = detail.error
  useEffect(() => {
    if (!userId || !detailError) return
    if (isApiError(detailError) && sessionPath(detailError.state)) return
    console.warn("同事资料加载失败", detailError)
    toast.error(t("detail.loadError"))
    onClose()
  }, [detailError, onClose, t, userId])

  return (
    <ContactDetailSheet
      open={Boolean(userId)}
      onClose={onClose}
      title={user?.displayName ?? t("detail.memberTitle")}
      description={t("detail.memberProfileDescription")}
      loading={detail.loading && Boolean(userId)}
    >
      {user ? (
        <div className="flex flex-col gap-7">
          <div className="flex items-center gap-3 px-2">
            <ProfileAvatar
              name={user.displayName}
              imageURL={user.avatarUrl}
              className="size-14 text-xl"
            />
            <div className="min-w-0 space-y-2">
              <p className="break-words text-base font-medium">{user.displayName}</p>
              {user.status === UserStatus.UserStatusActive && workStatus ? (
                <WorkStatusBadge status={workStatus} />
              ) : (
                <p className="text-sm text-muted-foreground">{t("statuses.inactive")}</p>
              )}
            </div>
          </div>
          <div className="divide-y">
            <ReadonlyDetailRow label={t("columns.email")}>
              <span className="break-all">{user.email}</span>
            </ReadonlyDetailRow>
            <ReadonlyDetailRow label={t("columns.teams")}>
              {user.teams.length
                ? user.teams.map((team) => (
                    <span key={team.id} className="block break-words">
                      {team.name}
                    </span>
                  ))
                : t("detail.empty")}
            </ReadonlyDetailRow>
          </div>
          {user.identityId !== identity.user.identityId ? (
            <div>
              <Button
                disabled={user.status !== UserStatus.UserStatusActive}
                onClick={() => navigate(`/chats?target=${user.identityId}`)}
              >
                {t("sendMessage")}
              </Button>
            </div>
          ) : null}
        </div>
      ) : null}
    </ContactDetailSheet>
  )
}
