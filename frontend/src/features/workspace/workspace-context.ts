/** 提供工作台页面共享数据。 */
import { createContext, createElement, useContext, type ReactNode } from "react"

import type {
  Identity,
  MessageNotificationInput,
  Organization,
  CurrentUser,
} from "@/api"

/** 工作台实时消息入口需要的通知内容和最新未读数。 */
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

const WorkspaceContext = createContext<WorkspaceOutletContext | null>(null)

/** 向长期挂载的工作台页面提供共享状态。 */
export function WorkspaceProvider({
  value,
  children,
}: {
  value: WorkspaceOutletContext
  children: ReactNode
}) {
  return createElement(WorkspaceContext.Provider, { value }, children)
}

/** 返回工作台子页面共享上下文。 */
export function useWorkspace() {
  const context = useContext(WorkspaceContext)
  if (!context) {
    throw new Error("工作台上下文不可用")
  }
  return context
}
