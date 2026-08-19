import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon, SmartphoneIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, Route, Routes, useNavigate } from "react-router"
import { toast } from "sonner"

import { logout, resolveNativeEntry, type Principal } from "@/api"
import { Button } from "@/components/ui/button"
import { LoginPage } from "@/features/auth/login-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"

function MobileHomePage() {
  const { t } = useTranslation("mobile")
  const navigate = useNavigate()
  const [principal, setPrincipal] = useState<Principal | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [loggingOut, setLoggingOut] = useState(false)

  const fetchSession = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const entry = await resolveNativeEntry()
      if (entry.status !== "ready") {
        navigate(entry.status === "connect" ? "/connect" : "/login", {
          replace: true,
        })
        return
      }
      setPrincipal(entry.principal)
    } catch {
      setError(t("loadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, t])

  useEffect(() => {
    void fetchSession()
  }, [fetchSession])

  async function handleLogout() {
    setLoggingOut(true)
    try {
      await logout()
    } catch {
      toast.error(t("logoutError"))
    } finally {
      setLoggingOut(false)
      navigate("/login", { replace: true })
    }
  }

  if (loading) {
    return (
      <main className="flex min-h-dvh items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </main>
    )
  }

  if (!principal) {
    return (
      <main className="flex min-h-dvh items-center justify-center p-6">
        <div className="text-center">
          <p className="text-sm text-muted-foreground">{error}</p>
          <Button className="mt-4" variant="outline" onClick={fetchSession}>
            {t("retry")}
          </Button>
        </div>
      </main>
    )
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
            <dd className="mt-1 font-medium">{principal.organization.name}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t("user")}</dt>
            <dd className="mt-1 font-medium">{principal.user.displayName}</dd>
            <dd className="text-muted-foreground">{principal.user.email}</dd>
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
