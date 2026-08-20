/** 登录页。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { getInstallationStatus } from "@/api"
import { LoginForm } from "@/features/auth/login-form"

/** 已连接企业时展示企业名称、登录表单，以及原生端修改服务器入口。 */
export function LoginPage({
  allowServerChange = false,
}: {
  allowServerChange?: boolean
}) {
  const { t } = useTranslation("auth")
  const navigate = useNavigate()
  const [organizationName, setOrganizationName] = useState("")

  useEffect(() => {
    const controller = new AbortController()
    const disconnectedPath = allowServerChange ? "/connect" : "/setup"

    void getInstallationStatus(controller.signal)
      .then((status) => {
        if (controller.signal.aborted) {
          return
        }
        const name = status.organizationName.trim()
        if (!status.installed || name === "") {
          navigate(disconnectedPath, { replace: true })
          return
        }
        setOrganizationName(name)
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          navigate(disconnectedPath, { replace: true })
        }
      })

    return () => controller.abort()
  }, [allowServerChange, navigate])

  if (organizationName === "") {
    return (
      <main className="flex min-h-svh items-center justify-center">
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
        <LoginForm allowServerChange={allowServerChange} />
      </div>
    </main>
  )
}
