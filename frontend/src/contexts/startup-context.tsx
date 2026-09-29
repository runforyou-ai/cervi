/** 提供登录方式、服务器连接状态和启动完成入口。 */
import { createContext, useContext, useMemo } from "react"

type StartupContextValue = {
  // 托管部署使用官方账号登录。
  usesOfficialLogin: boolean
  // 启动时已连接到可用的服务器。
  connected: boolean
  completeStartup: () => void
}

const StartupContext = createContext<StartupContextValue | null>(null)

/** 向启动流程内的页面提供启动状态。 */
export function StartupProvider({
  usesOfficialLogin,
  connected,
  completeStartup,
  children,
}: StartupContextValue & { children: React.ReactNode }) {
  const value = useMemo(
    () => ({ usesOfficialLogin, connected, completeStartup }),
    [usesOfficialLogin, connected, completeStartup],
  )
  return (
    <StartupContext.Provider value={value}>
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
