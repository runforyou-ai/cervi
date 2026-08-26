/** 登录页。 */
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, useNavigate } from "react-router"

import { LoginForm } from "@/features/auth/login-form"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import { useStartup } from "@/contexts/startup-context"

/** 检测已有登录身份并展示企业登录。 */
export function LoginPage({
  allowServerChange = false,
}: {
  allowServerChange?: boolean
}) {
  const { t } = useTranslation("auth")
  const navigate = useNavigate()
  const { organizationName } = useStartup()
  const { status, redirectPath } = useIdentityLoader()

  if (status === "loaded") return <Navigate to="/inbox" replace />
  if (status === "redirect" && redirectPath) {
    return <Navigate to={redirectPath} replace />
  }
  if (status === "loading") {
    return (
      <main className="flex min-h-dvh items-center justify-center">
        <LoaderCircleIcon className="size-4 animate-spin text-muted-foreground" />
      </main>
    )
  }
  if (organizationName.trim() === "") {
    return <Navigate to={allowServerChange ? "/connect" : "/setup"} replace />
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
