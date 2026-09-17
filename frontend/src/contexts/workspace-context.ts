/** 提供工作台页面共享数据。 */
import { createContext, createElement, useContext, type ReactNode } from "react"

import type { Identity } from "@/api"

export type WorkspaceOutletContext = {
  identity: Identity
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
