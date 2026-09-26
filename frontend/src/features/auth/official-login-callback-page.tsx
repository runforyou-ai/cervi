/** 官方账号授权回调页。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearchParams } from "react-router"

import { completeOfficialLogin, isApiError, OfficialLoginStateError } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { useStartup } from "@/contexts/startup-context"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 用授权回调参数完成官方账号登录后进入收件箱，失败时提示并提供重新登录。 */
export function OfficialLoginCallbackPage() {
  const { t } = useTranslation("auth")
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { organizationName } = useStartup()
  const [failure, setFailure] = useState<string | null>(null)
  const started = useRef(false)

  useEffect(() => {
    // 授权码只能交换一次，开发模式重复执行副作用时不再提交。
    if (started.current) return
    started.current = true
    const code = searchParams.get("code")
    const state = searchParams.get("state")
    if (searchParams.get("error") || !code || !state) {
      setFailure(t("officialDenied"))
      return
    }
    completeOfficialLogin(state, code)
      .then(() => navigate("/inbox", { replace: true }))
      .catch((error: unknown) => {
        if (error instanceof OfficialLoginStateError) {
          setFailure(t("officialExpired"))
          return
        }
        if (recoverSession(error, navigate)) {
          return
        }
        setFailure(isApiError(error) ? apiErrorMessage(error) : t("networkError"))
      })
  }, [navigate, searchParams, t])

  return (
    <main className="flex min-h-dvh w-full items-center justify-center px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))] md:p-10">
      <div className="w-full max-w-sm">
        <p className="mb-8 text-center text-xl font-medium tracking-tight">{organizationName}</p>
        {failure ? (
          <Card>
            <CardHeader>
              <CardTitle>{t("officialFailedTitle")}</CardTitle>
              <CardDescription>{failure}</CardDescription>
            </CardHeader>
            <CardContent>
              <Button type="button" className="w-full" onClick={() => navigate("/login", { replace: true })}>
                {t("signInAgain")}
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className="flex justify-center">
            <LoadingIndicator>
              <span className="text-sm text-muted-foreground">{t("submitting")}</span>
            </LoadingIndicator>
          </div>
        )}
      </div>
    </main>
  )
}
