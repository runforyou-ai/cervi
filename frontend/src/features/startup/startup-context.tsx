/** 提供企业名称和启动完成入口。 */
import { createContext, useContext } from "react"

type StartupContextValue = {
  organizationName: string
  completeStartup: (organizationName: string) => void
}

const StartupContext = createContext<StartupContextValue | null>(null)

/** 向启动流程内的页面提供启动状态。 */
export function StartupProvider({
  organizationName,
  completeStartup,
  children,
}: StartupContextValue & { children: React.ReactNode }) {
  return (
    <StartupContext.Provider value={{ organizationName, completeStartup }}>
      {children}
    </StartupContext.Provider>
  )
}

/** 返回启动流程状态。 */
export function useStartup() {
  const context = useContext(StartupContext)
  if (!context) throw new Error("useStartup 必须在 StartupProvider 内使用")
  return context
}
