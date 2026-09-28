/** 登录页，托管部署在 Web 端使用官方账号登录，自托管部署开放注册时提供注册入口。 */
import { useEffect } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { Link, Navigate, useNavigate } from "react-router"

import { isApiError, loadAccount, loadInstallationStatus, sessionPath, SessionState } from "@/api"
import { clearWebToken } from "@/api/client"
import { LoadingIndicator } from "@/components/loading-indicator"
import { LoginForm } from "@/features/auth/login-form"
import { OfficialLoginCard, OfficialLoginUnsupported } from "@/features/auth/official-login-card"
import { useStartup } from "@/contexts/startup-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { resolveServerURL } from "@/lib/server-url"
import { resolveAppPlatform } from "@/platform/app-platform"

/** 已登录时进入工作区入口，否则展示登录方式。 */
export function LoginPage({
  allowServerChange = false,
}: {
  allowServerChange?: boolean
}) {
  const { t } = useTranslation("auth")
  const navigate = useNavigate()
  const { usesOfficialLogin } = useStartup()
  const installation = useQuery({
    queryKey: resourceKeys.installationStatus(),
    queryFn: ({ signal }) => loadInstallationStatus(signal),
    staleTime: 0,
  })
  const registrationOpen = Boolean(installation.data?.registrationOpen)
  // 原生端展示已连接的部署，未配置部署名称时展示服务器地址。
  const serverURL = useResource(resourceKeys.serverURL(), () => resolveServerURL(), { enabled: allowServerChange })
  const deploymentLabel = installation.data?.deploymentName || (serverURL.data ? new URL(serverURL.data).host : "")
  const { data, error } = useQuery({
    queryKey: resourceKeys.account(),
    queryFn: ({ signal }) => loadAccount(signal),
  })
  const sessionState = isApiError(error) ? error.state : ""

  // 登录会话已失效时清除本地令牌。
  useEffect(() => {
    if (sessionState === SessionState.SessionStateLogin) clearWebToken()
  }, [sessionState])

  if (data) return <Navigate to="/" replace />
  if (sessionState && sessionState !== SessionState.SessionStateLogin) {
    const path = sessionPath(sessionState)
    if (path) return <Navigate to={path} replace />
  }
  if (!error) {
    return (
      <main className="flex min-h-dvh items-center justify-center">
        <LoadingIndicator>
          <span className="sr-only">Loading</span>
        </LoadingIndicator>
      </main>
    )
  }
  return (
    <main className="flex min-h-dvh w-full items-center justify-center px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))] md:p-10">
      <div className="w-full max-w-sm">
        <div className="mb-8 w-full">
          <p className="text-center text-xl font-medium tracking-tight">
            Cervi
            {allowServerChange ? (
              <button
                type="button"
                className="ml-2.5 inline-block whitespace-nowrap align-bottom text-xs font-medium tracking-[0.16em] text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => navigate("/connect")}
              >
                {t("changeServer")}
              </button>
            ) : null}
          </p>
          {allowServerChange && deploymentLabel ? (
            <p className="mt-1.5 truncate text-center text-sm text-muted-foreground">{deploymentLabel}</p>
          ) : null}
        </div>
        {usesOfficialLogin ? (
          resolveAppPlatform() === "web" ? <OfficialLoginCard /> : <OfficialLoginUnsupported />
        ) : (
          <LoginForm />
        )}
        {!usesOfficialLogin && registrationOpen ? (
          <p className="mt-6 text-center text-sm text-muted-foreground">
            {t("registerPrompt")}{" "}
            <Link to="/register" className="font-medium text-foreground underline-offset-4 hover:underline">
              {t("registerLink")}
            </Link>
          </p>
        ) : null}
      </div>
    </main>
  )
}
