/** 登录页。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { getInstallationStatus } from "@/api"
import { Button } from "@/components/ui/button"
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
    <main className="flex min-h-svh w-full items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <p className="text-lg font-semibold tracking-tight">{organizationName}</p>
          {allowServerChange ? (
            <Button
              type="button"
              variant="link"
              size="sm"
              className="h-auto px-0 text-muted-foreground"
              onClick={() => navigate("/connect")}
            >
              {t("changeServer")}
            </Button>
          ) : null}
        </div>
        <LoginForm allowServerChange={allowServerChange} />
      </div>
    </main>
  )
}
