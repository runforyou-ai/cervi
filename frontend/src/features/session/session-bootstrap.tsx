/** 在业务路由挂载前确定唯一的初始会话入口。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { Navigate, useLocation } from "react-router"

import { SessionState, type Session } from "@/api"
import { SessionLoadFailedState } from "@/features/session/session-load-failed-state"
import { useSessionLoader } from "@/features/session/use-session-loader"
import type { AppPlatform } from "@/platform/app-platform"

/** 判断路径是否为会话入口。 */
function isEntrancePath(pathname: string) {
  return (
    pathname === "/login" ||
    pathname === "/connect" ||
    pathname === "/setup"
  )
}

/** 根据平台和权威会话状态选择初始路径。 */
function resolveSessionPath(
  platform: AppPlatform,
  session: Session,
  pathname: string,
) {
  if (session.state === SessionState.SessionStateReady) {
    if (!session.identity) return null
    return pathname === "/" || isEntrancePath(pathname) ? "/inbox" : pathname
  }
  if (session.state === SessionState.SessionStateLogin) {
    if (!session.organizationName?.trim()) {
      return platform === "web" ? "/setup" : "/connect"
    }
    return "/login"
  }
  if (
    session.state === SessionState.SessionStateSetup ||
    session.state === SessionState.SessionStateConnect
  ) {
    return platform === "web" ? "/setup" : "/connect"
  }
  return null
}

/** 首次会话入口确定后透传现有路由和页面级会话处理。 */
export function SessionBootstrap({
  platform,
  children,
}: {
  platform: AppPlatform
  children: React.ReactNode
}) {
  const location = useLocation()
  const { status, session, retry } = useSessionLoader()
  const [forcedPath, setForcedPath] = useState<string | null>(null)
  const [completed, setCompleted] = useState(false)
  const targetPath =
    forcedPath ??
    (status === "loaded" && session
      ? resolveSessionPath(platform, session, location.pathname)
      : null)

  useEffect(() => {
    if (targetPath && location.pathname === targetPath) {
      setCompleted(true)
    }
  }, [location.pathname, targetPath])

  if (completed) {
    return children
  }
  if (targetPath) {
    return location.pathname === targetPath ? (
      children
    ) : (
      <Navigate to={targetPath} replace />
    )
  }
  if (status === "loading") {
    return (
      <main className="flex min-h-dvh items-center justify-center">
        <LoaderCircleIcon
          aria-label="Loading"
          className="size-5 animate-spin text-muted-foreground"
        />
      </main>
    )
  }
  return (
    <SessionLoadFailedState
      onRetry={retry}
      onChangeServer={
        platform === "web" ? undefined : () => setForcedPath("/connect")
      }
    />
  )
}
