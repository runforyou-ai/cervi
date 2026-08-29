/** 登录页。 */
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, useNavigate } from "react-router"

import { LoginForm } from "@/features/auth/login-form"
import { Button } from "@/components/ui/button"
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
  const { status, redirectPath, retry } = useIdentityLoader()

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
  if (status === "failed") {
    return (
      <main className="flex min-h-dvh items-center justify-center px-6 text-center">
        <div>
          <p className="text-sm text-muted-foreground">
            {t("identityLoadError")}
          </p>
          <Button className="mt-4" variant="outline" onClick={retry}>
            {t("retry")}
          </Button>
        </div>
      </main>
    )
  }
  if (organizationName.trim() === "") {
    return <Navigate to={allowServerChange ? "/connect" : "/setup"} replace />
  }

  return (
    <main className="h-dvh w-full overflow-y-auto px-6 md:px-10">
      <div className="mx-auto flex min-h-full w-full max-w-sm flex-col justify-center pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))] md:py-10">
        <div className="mb-8 w-full">
          <p className="text-center text-xl font-medium tracking-tight">
            {organizationName}
            {allowServerChange ? (
              <button
                type="button"
                className="ml-2.5 inline-block whitespace-nowrap align-bottom text-[11px] font-medium tracking-[0.16em] text-muted-foreground transition-colors hover:text-foreground"
                onClick={() =>
                  navigate("/connect", {
                    replace: true,
                    state: { from: "login" },
                  })
                }
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
