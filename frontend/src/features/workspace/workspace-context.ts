/** 工作台子页面共享的当前身份和用户更新入口。 */
import { useOutletContext } from "react-router"

import type {
  Identity,
  MessageNotificationInput,
  Organization,
  CurrentUser,
} from "@/api"

/** 工作台实时消息入口需要的通知内容和最新绝对未读数。 */
export type WorkspaceNewMessageNotification = Omit<
  MessageNotificationInput,
  "soundEnabled"
> & { unreadCount: number }

export type WorkspaceOutletContext = {
  identity: Identity
  updateOrganization: (organization: Organization) => void
  updateUser: (user: CurrentUser) => void
  beginUnreadSnapshot: () => number
  applyUnreadSnapshot: (count: number, revision: number) => void
  notifyNewMessage: (
    notification: WorkspaceNewMessageNotification,
  ) => Promise<boolean>
}

/** 返回工作台子页面共享上下文。 */
export function useWorkspace() {
  return useOutletContext<WorkspaceOutletContext>()
}
