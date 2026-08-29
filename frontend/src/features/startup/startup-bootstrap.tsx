/** 在业务路由挂载前完成统一启动检测。 */
import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation } from "react-router"

import { SessionState, type Startup } from "@/api"
import { Button } from "@/components/ui/button"
import { StartupProvider } from "@/contexts/startup-context"
import { useStartupLoader } from "@/features/startup/use-startup-loader"

/** 根据启动状态选择连接、初始化或当前应用入口。 */
function resolveStartupPath(startup: Startup, pathname: string) {
  if (startup.state === SessionState.SessionStateSetup) return "/setup"
  if (startup.state === SessionState.SessionStateConnect) return "/connect"
  if (startup.state === SessionState.SessionStateReady) {
    return pathname === "/setup" || pathname === "/connect" ? "/" : pathname
  }
  return null
}

/** 展示启动检测期间的占位界面。 */
function StartupLoading() {
  return (
    <main className="flex min-h-dvh items-center justify-center">
      <LoaderCircleIcon
        aria-label="Loading"
        className="size-5 animate-spin text-muted-foreground"
      />
    </main>
  )
}

/** 展示启动检测失败并允许重试。 */
function StartupFailed({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation("common")

  return (
    <main className="flex min-h-dvh items-center justify-center px-6 text-center">
      <div>
        <p className="text-sm text-muted-foreground">
          {t("startupLoadError")}
        </p>
        <Button className="mt-4" variant="outline" onClick={onRetry}>
          {t("retry")}
        </Button>
      </div>
    </main>
  )
}

/** 启动检测完成前阻止业务页面挂载。 */
export function StartupBootstrap({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { status, startup, retry } = useStartupLoader()
  const [completed, setCompleted] = useState(false)
  const [organizationName, setOrganizationName] = useState<string | null>(null)
  const completeStartup = useCallback((name: string) => {
    setOrganizationName(name)
    setCompleted(true)
  }, [])

  const currentOrganizationName =
    organizationName ?? startup?.organizationName ?? ""
  const content = (
    <StartupProvider
      organizationName={currentOrganizationName}
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
    return <StartupFailed onRetry={retry} />
  }
  if (status === "loading") {
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
