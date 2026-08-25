/** 在知识库页面间共享窄边栏列表同步入口。 */
import { createContext, useContext, type ReactNode } from "react"

import type { KnowledgeBaseData } from "@/api"

type KnowledgeBaseContextValue = {
  upsertKnowledgeBase: (knowledgeBase: KnowledgeBaseData) => void
}

const KnowledgeBaseContext = createContext<KnowledgeBaseContextValue | null>(
  null,
)

/** 向知识库子页面提供窄边栏列表同步入口。 */
export function KnowledgeBaseProvider({
  upsertKnowledgeBase,
  children,
}: KnowledgeBaseContextValue & { children: ReactNode }) {
  return (
    <KnowledgeBaseContext.Provider value={{ upsertKnowledgeBase }}>
      {children}
    </KnowledgeBaseContext.Provider>
  )
}

/** 返回知识库窄边栏列表同步入口。 */
export function useKnowledgeBaseContext() {
  const context = useContext(KnowledgeBaseContext)
  if (!context) {
    throw new Error("useKnowledgeBaseContext 必须在 KnowledgeBaseProvider 内使用")
  }
  return context
}
