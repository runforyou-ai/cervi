/** 登录页。 */
import { useEffect } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { sessionPath, SessionState } from "@/api"
import { LoginForm } from "@/features/auth/login-form"
import { SessionLoadFailedState } from "@/features/session/session-load-failed-state"
import { useSessionLoader } from "@/features/session/use-session-loader"

/** 解析登录入口，并展示企业登录或服务器错误状态。 */
export function LoginPage({
  allowServerChange = false,
}: {
  allowServerChange?: boolean
}) {
  const { t } = useTranslation("auth")
  const navigate = useNavigate()
  const { status, session, retry } = useSessionLoader()
  const disconnectedPath = allowServerChange ? "/connect" : "/setup"

  useEffect(() => {
    if (status !== "loaded" || !session) {
      return
    }
    if (session.state === SessionState.SessionStateReady) {
      navigate("/inbox", { replace: true })
      return
    }
    if (session.state !== SessionState.SessionStateLogin) {
      navigate(sessionPath(session.state) ?? disconnectedPath, {
        replace: true,
      })
    }
  }, [disconnectedPath, navigate, session, status])

  if (status === "failed") {
    return (
      <SessionLoadFailedState
        onRetry={retry}
        onChangeServer={
          allowServerChange
            ? () => navigate("/connect", { replace: true })
            : undefined
        }
      />
    )
  }

  const organizationName = session?.organizationName?.trim() ?? ""

  if (
    status !== "loaded" ||
    session?.state !== SessionState.SessionStateLogin ||
    organizationName === ""
  ) {
    return (
      <main className="flex min-h-dvh items-center justify-center">
        <LoaderCircleIcon className="size-4 animate-spin text-muted-foreground" />
      </main>
    )
  }

  return (
    <main className="flex min-h-dvh w-full items-center justify-center px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))] md:p-10">
      <div className="w-full max-w-sm">
        <div className="mb-8 w-full">
          <p className="text-center text-xl font-medium tracking-tight">
            {organizationName}
            {allowServerChange ? (
              <button
                type="button"
                className="ml-2.5 inline-block whitespace-nowrap align-bottom text-[11px] font-medium tracking-[0.16em] text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => navigate("/connect")}
              >
                {t("changeServer")}
              </button>
            ) : null}
          </p>
        </div>
        <LoginForm />
      </div>
    </main>
  )
}
