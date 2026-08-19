import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Outlet, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import { loadSession, logout } from "@/api/auth"
import { ApiError, hasSession } from "@/api/client"
import type { Principal } from "@/api/identity"
import { getServerURL } from "@/api/server-connection"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"

function workspaceTitle(
  pathname: string,
  titles: {
    inbox: string
    createWebsiteChannel: string
    websiteChannelTrash: string
    editWebsiteChannel: string
    websiteChannels: string
    settings: string
    contacts: string
  }
) {
  if (pathname.startsWith("/settings")) {
    return titles.settings
  }
  if (pathname.startsWith("/contacts")) {
    return titles.contacts
  }
  if (pathname === "/channels/website/new") {
    return titles.createWebsiteChannel
  }
  if (pathname === "/channels/website/trash") {
    return titles.websiteChannelTrash
  }
  if (pathname.startsWith("/channels/website/")) {
    return titles.editWebsiteChannel
  }
  if (pathname.startsWith("/channels/website")) {
    return titles.websiteChannels
  }
  return titles.inbox
}

export function WorkspaceLayout({
  platform,
}: {
  platform: "web" | "desktop"
}) {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const location = useLocation()
  const [principal, setPrincipal] = useState<Principal | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [loggingOut, setLoggingOut] = useState(false)

  const fetchSession = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      if (platform === "desktop") {
        if ((await getServerURL()) === "") {
          navigate("/connect", { replace: true })
          return
        }
        if (!hasSession()) {
          navigate("/login", { replace: true })
          return
        }
      }
      setPrincipal(await loadSession())
    } catch (requestError) {
      if (requestError instanceof ApiError) {
        if (requestError.code === "SERVER_CONNECTION_REQUIRED") {
          navigate("/connect", { replace: true })
          return
        }
        if (requestError.code === "INSTALLATION_REQUIRED") {
          navigate(platform === "web" ? "/setup" : "/connect", {
            replace: true,
          })
          return
        }
        if (requestError.code === "AUTH_REQUIRED") {
          navigate("/login", { replace: true })
          return
        }
      }
      setError(t("loadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, platform, t])

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

  if (loading && !principal) {
    return (
      <main className="flex min-h-svh items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </main>
    )
  }

  if (!principal) {
    return (
      <main className="flex min-h-svh items-center justify-center p-6">
        <div className="text-center">
          <p className="text-sm text-muted-foreground">
            {error || t("loadError")}
          </p>
          <Button className="mt-4" variant="outline" onClick={fetchSession}>
            <RefreshCwIcon />
            {t("retry")}
          </Button>
        </div>
      </main>
    )
  }

  return (
    <SidebarProvider>
      <WorkspaceNavigation
        principal={principal}
        onLogout={handleLogout}
        loggingOut={loggingOut}
      />
      <SidebarInset className="h-svh min-h-0 min-w-0 overflow-hidden md:h-[calc(100svh-1rem)]">
        <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="mr-2 h-4" />
          <h1 className="text-sm font-medium">
            {workspaceTitle(location.pathname, {
              inbox: t("inbox"),
              createWebsiteChannel: t("titles.createWebsiteChannel"),
              websiteChannelTrash: t("titles.websiteChannelTrash"),
              editWebsiteChannel: t("titles.editWebsiteChannel"),
              websiteChannels: t("titles.websiteChannels"),
              settings: t("settings"),
              contacts: t("contacts"),
            })}
          </h1>
        </header>
        <div className="flex min-h-0 flex-1 overflow-auto">
          <Outlet context={principal} />
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}
