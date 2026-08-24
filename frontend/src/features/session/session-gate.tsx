/** 把平台、会话快照和当前位置投影为唯一应用入口。 */
import { useRef } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { Navigate, useLocation } from "react-router"

import { SessionState } from "@/api"
import { SessionLoadFailedState } from "@/features/session/session-load-failed-state"
import {
  useSessionController,
  useSessionSnapshot,
} from "@/features/session/session-context"
import type { AppPlatform } from "@/platform/app-platform"

/** 展示应用级稳定加载画面。 */
export function SessionLoadingState() {
  return (
    <main className="flex min-h-dvh items-center justify-center">
      <LoaderCircleIcon
        aria-label="Loading"
        className="size-5 animate-spin text-muted-foreground"
      />
    </main>
  )
}

/** 判断当前位置是否属于当前平台的会话入口。 */
function isEntrancePath(pathname: string) {
  return (
    pathname === "/login" ||
    pathname === "/connect" ||
    pathname === "/setup"
  )
}

/** 在会话确定前阻止业务路由挂载，并声明式同步最终入口。 */
export function SessionGate({
  platform,
  children,
}: {
  platform: AppPlatform
  children: React.ReactNode
}) {
  const location = useLocation()
  const controller = useSessionController()
  const snapshot = useSessionSnapshot()
  const returnTo = useRef<string | null>(null)

  if (snapshot.status === "loading") {
    return <SessionLoadingState />
  }

  if (snapshot.status === "failed" || !snapshot.session) {
    return (
      <SessionLoadFailedState
        onRetry={() => void controller.reload("retry")}
        onChangeServer={
          platform === "web"
            ? undefined
            : () => controller.commitClassified(SessionState.SessionStateConnect)
        }
      />
    )
  }

  const { session } = snapshot
  if (session.state === SessionState.SessionStateConnect) {
    returnTo.current = null
    const target = platform === "web" ? "/setup" : "/connect"
    return location.pathname === target ? (
      children
    ) : (
      <Navigate to={target} replace />
    )
  }
  if (session.state === SessionState.SessionStateSetup) {
    returnTo.current = null
    const target = platform === "web" ? "/setup" : "/connect"
    return location.pathname === target ? (
      children
    ) : (
      <Navigate to={target} replace />
    )
  }
  if (session.state === SessionState.SessionStateLogin) {
    if (!session.organizationName?.trim()) {
      return (
        <SessionLoadFailedState
          onRetry={() => void controller.reload("retry")}
          onChangeServer={
            platform === "web"
              ? undefined
              : () =>
                  controller.commitClassified(
                    SessionState.SessionStateConnect,
                  )
          }
        />
      )
    }
    if (!isEntrancePath(location.pathname) && location.pathname !== "/") {
      returnTo.current = `${location.pathname}${location.search}${location.hash}`
    }
    return location.pathname === "/login" ? (
      children
    ) : (
      <Navigate to="/login" replace />
    )
  }
  if (
    session.state !== SessionState.SessionStateReady ||
    !session.identity
  ) {
    return (
      <SessionLoadFailedState
        onRetry={() => void controller.reload("retry")}
        onChangeServer={
          platform === "web"
            ? undefined
            : () => controller.commitClassified(SessionState.SessionStateConnect)
        }
      />
    )
  }

  if (
    location.pathname === "/" ||
    isEntrancePath(location.pathname) ||
    (platform === "mobile" && location.pathname !== "/inbox")
  ) {
    const target = returnTo.current ?? "/inbox"
    returnTo.current = null
    return <Navigate to={target} replace />
  }

  return children
}
