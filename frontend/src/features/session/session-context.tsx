/** 向组件树注入显式创建的会话控制器。 */
import { createContext, useContext, useSyncExternalStore } from "react"

import { SessionController } from "@/features/session/session-controller"

const SessionContext = createContext<SessionController | null>(null)

/** 提供当前应用实例的会话控制器。 */
export function SessionProvider({
  controller,
  children,
}: {
  controller: SessionController
  children: React.ReactNode
}) {
  return (
    <SessionContext.Provider value={controller}>
      {children}
    </SessionContext.Provider>
  )
}

/** 返回当前应用实例的会话控制器。 */
export function useSessionController() {
  const controller = useContext(SessionContext)
  if (!controller) {
    throw new Error("SessionProvider 未挂载")
  }
  return controller
}

/** 订阅当前会话快照。 */
export function useSessionSnapshot() {
  const controller = useSessionController()
  return useSyncExternalStore(
    controller.subscribe,
    controller.getSnapshot,
    controller.getSnapshot,
  )
}
