/** 移动端群聊与详情子页共享的资料和操作状态。 */
import { useOutletContext } from "react-router"
import type { GroupConversationData } from "@/api"

/** 移动端群聊的参与人数上限。 */
export const mobileGroupMemberLimit = 100

export type MobileGroupContext = {
  group: GroupConversationData
  returnDepth: number
  error: unknown
  refreshing: boolean
  refresh: () => Promise<unknown>
  onUnavailable: () => void
  onLeft: () => void
  onLeavingChange: (leaving: boolean) => void
}

export type MobileGroupDetailsContext = {
  group: GroupConversationData
  returnDepth: number
  canManage: boolean
  busy: boolean
  onSave: (action: () => Promise<unknown>) => Promise<boolean>
}

/** 读取群聊父路由维护的服务端资料。 */
export function useMobileGroup() {
  return useOutletContext<MobileGroupContext>()
}
