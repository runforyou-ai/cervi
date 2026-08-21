/** 工作台子页面共享的当前身份和用户更新入口。 */
import { useOutletContext } from "react-router"

import type { Identity, User } from "@/api"

export type WorkspaceOutletContext = {
  identity: Identity
  updateUser: (user: User) => void
}

/** 返回工作台子页面共享上下文。 */
export function useWorkspace() {
  return useOutletContext<WorkspaceOutletContext>()
}
