/** 登录页。 */
import { useTranslation } from "react-i18next"

import { SessionState } from "@/api"
import { LoginForm } from "@/features/auth/login-form"
import {
  useSessionController,
  useSessionSnapshot,
} from "@/features/session/session-context"

/** 解析登录入口，并展示企业登录或服务器错误状态。 */
export function LoginPage({
  allowServerChange = false,
}: {
  allowServerChange?: boolean
}) {
  const { t } = useTranslation("auth")
  const controller = useSessionController()
  const { session } = useSessionSnapshot()
  const organizationName = session?.organizationName?.trim() ?? ""

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
                onClick={() =>
                  controller.commitClassified(SessionState.SessionStateConnect)
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
