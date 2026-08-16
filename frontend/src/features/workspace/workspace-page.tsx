import { useCallback, useEffect, useState } from "react"
import {
  ChevronsUpDownIcon,
  InboxIcon,
  LoaderCircleIcon,
  LogOutIcon,
  PanelsTopLeftIcon,
  RefreshCwIcon,
  SettingsIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Outlet,
  useLocation,
  useNavigate,
  useOutletContext,
} from "react-router"
import { toast } from "sonner"

import { logout } from "@/api/auth"
import { ApiError } from "@/api/client"
import { loadInbox, type InboxData } from "@/api/inbox"
import { InboxPage } from "@/features/inbox/inbox-page"
import { WorkspacePageHeader } from "@/features/workspace/workspace-page-header"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  useSidebar,
} from "@/components/ui/sidebar"

function WorkspaceNavigation({
  data,
  onLogout,
  loggingOut,
}: {
  data: InboxData
  onLogout: () => void
  loggingOut: boolean
}) {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const location = useLocation()
  const { setOpenNarrowViewport } = useSidebar()
  const [userMenuOpen, setUserMenuOpen] = useState(false)

  function openInbox() {
    if (location.pathname !== "/inbox") {
      navigate("/inbox")
    }
    setOpenNarrowViewport(false)
  }

  function openSettings() {
    if (location.pathname !== "/settings") {
      navigate("/settings")
    }
    setOpenNarrowViewport(false)
  }

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" tooltip="Cervi">
              <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                <PanelsTopLeftIcon className="size-4" />
              </div>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-semibold">Cervi</span>
                <span className="truncate text-xs">
                  {data.organization.name}
                </span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>{t("navigationGroup")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton
                  isActive={location.pathname === "/inbox"}
                  tooltip={t("inbox")}
                  onClick={openInbox}
                >
                  <InboxIcon />
                  <span>{t("inbox")}</span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu
              open={userMenuOpen}
              onOpenChange={setUserMenuOpen}
            >
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  size="lg"
                  tooltip={userMenuOpen ? undefined : data.user.displayName}
                  aria-label={t("openUserMenu", {
                    name: data.user.displayName,
                  })}
                >
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-xs font-semibold">
                    {data.user.displayName.slice(0, 1).toUpperCase()}
                  </div>
                  <div className="grid min-w-0 flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">
                      {data.user.displayName}
                    </span>
                    <span className="truncate text-xs text-muted-foreground">
                      {data.user.email}
                    </span>
                  </div>
                  <ChevronsUpDownIcon className="ml-auto" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                side="top"
                align="start"
                className="w-56"
              >
                <DropdownMenuLabel className="font-normal">
                  <div className="grid gap-0.5 leading-tight">
                    <span className="truncate font-medium">
                      {data.user.displayName}
                    </span>
                    <span className="truncate text-xs text-muted-foreground">
                      {data.user.email}
                    </span>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={openSettings}>
                  <SettingsIcon />
                  {t("settings")}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  destructive
                  disabled={loggingOut}
                  onSelect={(event) => {
                    event.preventDefault()
                    onLogout()
                  }}
                >
                  {loggingOut ? (
                    <LoaderCircleIcon className="animate-spin" />
                  ) : (
                    <LogOutIcon />
                  )}
                  {loggingOut ? t("loggingOut") : t("logout")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  )
}

export function WorkspacePage() {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const [data, setData] = useState<InboxData | null>(null)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(true)
  const [loggingOut, setLoggingOut] = useState(false)

  const fetchInbox = useCallback(async () => {
    setLoading(true)
    setError("")

    try {
      setData(await loadInbox())
    } catch (requestError) {
      if (requestError instanceof ApiError) {
        if (requestError.code === "SERVER_CONNECTION_REQUIRED") {
          navigate("/connect", { replace: true })
          return
        }
        if (requestError.code === "INSTALLATION_REQUIRED") {
          navigate("/setup", { replace: true })
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
  }, [navigate, t])

  useEffect(() => {
    void fetchInbox()
  }, [fetchInbox])

  async function handleLogout() {
    setLoggingOut(true)
    try {
      await logout()
      navigate("/login", { replace: true })
    } catch (error) {
      if (error instanceof ApiError && error.code === "AUTH_REQUIRED") {
        navigate("/login", { replace: true })
        return
      }
      toast.error(t("logoutError"))
    } finally {
      setLoggingOut(false)
    }
  }

  if (loading && !data) {
    return (
      <main className="flex min-h-svh items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </main>
    )
  }

  if (!data) {
    return (
      <main className="flex min-h-svh items-center justify-center p-6">
        <div className="text-center">
          <p className="text-sm text-muted-foreground">
            {error || t("loadError")}
          </p>
          <Button className="mt-4" variant="outline" onClick={fetchInbox}>
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
        data={data}
        onLogout={handleLogout}
        loggingOut={loggingOut}
      />
      <SidebarInset className="h-svh min-h-0 min-w-0 overflow-hidden md:h-[calc(100svh-1rem)]">
        <Outlet context={data} />
      </SidebarInset>
    </SidebarProvider>
  )
}

export function WorkspaceInboxPage() {
  const { t } = useTranslation("workspace")
  const data = useOutletContext<InboxData>()

  return (
    <>
      <WorkspacePageHeader title={t("inbox")} />
      <InboxPage conversations={data.conversations} />
    </>
  )
}
