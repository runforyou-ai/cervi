/** 设置中企业成员的可编辑详情侧滑面板。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  getUser,
  isApiError,
  sessionPath,
  type RoleData,
  type Team,
} from "@/api"
import { useWorkspace } from "@/contexts/workspace-context"
import { ContactDetailSheet } from "@/features/contacts/contact-detail-sheet"
import { MemberDetailView } from "@/features/settings/members/member-detail"
import { useContactInvalidator } from "@/features/contacts/use-contact-invalidator"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

/** 按用户编号加载并展示成员详情，读取失败或成员不存在时关闭。 */
export function MemberDetailSheet({
  userId,
  roles,
  teams,
  onClose,
}: {
  userId: string
  roles: RoleData[]
  teams: Team[]
  onClose: () => void
}) {
  const { t } = useTranslation("contacts")
  const { identity } = useWorkspace()
  const invalidateContact = useContactInvalidator()
  const invalidate = useResourceInvalidator()
  const detail = useResource(resourceKeys.user(userId), () => getUser(userId), {
    enabled: Boolean(userId),
  })
  const user = userId ? (detail.data ?? null) : null

  const detailError = detail.error
  useEffect(() => {
    if (!userId || !detailError) return
    if (isApiError(detailError) && sessionPath(detailError.state)) return
    console.warn("联系人详情加载失败", detailError)
    toast.error(t("detail.loadError"))
    onClose()
  }, [detailError, onClose, t, userId])

  return (
    <ContactDetailSheet
      open={Boolean(userId)}
      onClose={onClose}
      title={user?.displayName ?? t("detail.memberTitle")}
      description={t("detail.memberDescription")}
      loading={detail.loading && Boolean(userId)}
    >
      {user ? (
        <MemberDetailView
          key={user.id}
          user={user}
          teams={teams}
          roles={roles}
          // 自己的工作状态以当前身份为准，列表数据可能尚未刷新。
          workStatus={
            user.id === identity.user.id ? identity.user.workStatus : user.workStatus
          }
          onSaved={(saved) => {
            void invalidateContact("user", saved.id)
            if (saved.id === identity.user.id) {
              void invalidate(resourceKeys.identity())
            }
          }}
          onNotFound={() => {
            onClose()
            void invalidateContact("user")
          }}
        />
      ) : null}
    </ContactDetailSheet>
  )
}
