/** 在业务路由挂载前完成统一启动检测。 */
import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation } from "react-router"

import { DeploymentMode, SessionState, type Startup } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageLoadError } from "@/components/page-load-error"
import { StartupProvider } from "@/contexts/startup-context"
import { markStartupReady, useStartupLoader } from "@/features/startup/use-startup-loader"

/** 根据启动状态选择连接、初始化或当前应用入口；已就绪时仍可停在连接页切换服务器。 */
function resolveStartupPath(startup: Startup, pathname: string) {
  if (startup.state === SessionState.SessionStateSetup) return "/setup"
  if (startup.state === SessionState.SessionStateConnect) return "/connect"
  if (startup.state === SessionState.SessionStateReady) {
    return pathname === "/setup" ? "/" : pathname
  }
  return null
}

/** 展示启动检测期间的占位界面。 */
function StartupLoading() {
  return (
    <main className="flex min-h-dvh items-center justify-center">
      <LoadingIndicator>
        <span className="sr-only">Loading</span>
      </LoadingIndicator>
    </main>
  )
}

/** 启动检测完成前阻止业务页面挂载。 */
export function StartupBootstrap({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation("common")
  const location = useLocation()
  const { status, startup, retry } = useStartupLoader()
  const [completed, setCompleted] = useState(false)
  const completeStartup = useCallback(() => {
    markStartupReady()
    setCompleted(true)
  }, [])

  const content = (
    <StartupProvider
      usesOfficialLogin={startup?.deploymentMode === DeploymentMode.DeploymentModeManaged}
      connected={startup?.state === SessionState.SessionStateReady}
      completeStartup={completeStartup}
    >
      {children}
    </StartupProvider>
  )

  useEffect(() => {
    if (
      startup?.state === SessionState.SessionStateReady &&
      resolveStartupPath(startup, location.pathname) === location.pathname
    ) {
      setCompleted(true)
    }
  }, [location.pathname, startup])

  if (status === "failed") {
    return <PageLoadError message={t("status.serverUnavailable")} onRetry={retry} />
  }
  if (status !== "loaded") {
    return <StartupLoading />
  }
  if (completed) return content
  const targetPath = resolveStartupPath(startup, location.pathname)
  if (!targetPath) return <StartupLoading />
  return targetPath === location.pathname ? (
    content
  ) : (
    <Navigate to={targetPath} replace />
  )
}
