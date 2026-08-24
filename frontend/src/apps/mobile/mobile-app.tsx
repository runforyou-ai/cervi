/** 移动端独立入口、路由和首页。 */
import { useState } from "react"
import { LoaderCircleIcon, SmartphoneIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, Route, Routes } from "react-router"
import { toast } from "sonner"

import { logout } from "@/api"
import { Button } from "@/components/ui/button"
import { LoginPage } from "@/features/auth/login-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import {
  useSessionController,
  useSessionSnapshot,
} from "@/features/session/session-context"

/** 移动端登录后首页。 */
function MobileHomePage() {
  const { t } = useTranslation("mobile")
  const controller = useSessionController()
  const { session } = useSessionSnapshot()
  const identity = session!.identity!
  const [loggingOut, setLoggingOut] = useState(false)

  /** 退出登录并回到登录页。 */
  async function handleLogout() {
    setLoggingOut(true)
    try {
      await logout()
    } catch {
      toast.error(t("logoutError"))
    } finally {
      setLoggingOut(false)
      await controller.reload("logout")
    }
  }

  return (
    <main className="flex min-h-dvh items-center justify-center px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))]">
      <section className="max-w-sm text-center">
        <div className="mx-auto mb-5 flex size-12 items-center justify-center rounded-2xl border bg-background shadow-sm">
          <SmartphoneIcon className="size-5 text-muted-foreground" />
        </div>
        <p className="mb-2 text-sm font-semibold tracking-wide">Cervi · 鹿行</p>
        <h1 className="text-xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          {t("description")}
        </p>
        <dl className="mt-6 grid gap-4 rounded-lg border p-4 text-left text-sm">
          <div>
            <dt className="text-muted-foreground">{t("organization")}</dt>
            <dd className="mt-1 font-medium">{identity.organization.name}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t("user")}</dt>
            <dd className="mt-1 font-medium">{identity.user.displayName}</dd>
            <dd className="text-muted-foreground">{identity.user.email}</dd>
          </div>
        </dl>
        <Button className="mt-6" disabled={loggingOut} onClick={handleLogout}>
          {loggingOut ? <LoaderCircleIcon className="animate-spin" /> : null}
          {loggingOut ? t("loggingOut") : t("logout")}
        </Button>
      </section>
    </main>
  )
}

/** 渲染移动端路由。 */
export default function MobileApp() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      <Route path="/connect" element={<ServerConnectionPage />} />
      <Route path="/login" element={<LoginPage allowServerChange />} />
      <Route path="/inbox" element={<MobileHomePage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
