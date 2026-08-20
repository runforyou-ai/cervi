/** 工作台侧栏导航和用户菜单。 */
import { useState } from "react"
import {
  ChevronsUpDownIcon,
  ContactRoundIcon,
  InboxIcon,
  LoaderCircleIcon,
  LogOutIcon,
  MessagesSquareIcon,
  PanelsTopLeftIcon,
  SettingsIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { NavLink, useLocation, useNavigate } from "react-router"

import type { Identity } from "@/api"
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
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar"

/** 工作台主导航。 */
function WorkspaceMenu() {
  const { t } = useTranslation("workspace")
  const location = useLocation()
  const { isNarrowViewport, setOpenNarrowViewport } = useSidebar()
  const inboxActive = location.pathname === "/inbox"
  const contactsActive = location.pathname.startsWith("/contacts")
  const channelsActive = location.pathname.startsWith("/channels")

  /** 窄视口下关闭侧栏。 */
  function closeNarrowNavigation() {
    if (isNarrowViewport) {
      setOpenNarrowViewport(false)
    }
  }

  return (
    <SidebarGroup>
      <SidebarGroupLabel>{t("navigationGroup")}</SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild isActive={inboxActive} tooltip={t("inbox")}>
              <NavLink to="/inbox" onClick={closeNarrowNavigation}>
                <InboxIcon />
                <span>{t("inbox")}</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>

          <SidebarMenuItem>
            <SidebarMenuButton
              asChild
              isActive={contactsActive}
              tooltip={t("contacts")}
            >
              <NavLink to="/contacts/internal" onClick={closeNarrowNavigation}>
                <ContactRoundIcon />
                <span>{t("contacts")}</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>

          <SidebarMenuItem>
            <SidebarMenuButton
              asChild
              isActive={channelsActive}
              tooltip={t("messageChannels")}
            >
              <NavLink to="/channels/website" onClick={closeNarrowNavigation}>
                <MessagesSquareIcon />
                <span>{t("messageChannels")}</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

/** 工作台侧栏和用户菜单。 */
export function WorkspaceNavigation({
  identity,
  onLogout,
  loggingOut,
}: {
  identity: Identity
  onLogout: () => void
  loggingOut: boolean
}) {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const { setOpenNarrowViewport } = useSidebar()
  const [userMenuOpen, setUserMenuOpen] = useState(false)

  /** 打开设置页。 */
  function openSettings() {
    navigate("/settings/storage")
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
                  {identity.organization.name}
                </span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <WorkspaceMenu />
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
                  tooltip={userMenuOpen ? undefined : identity.user.displayName}
                  aria-label={t("openUserMenu", {
                    name: identity.user.displayName,
                  })}
                >
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-xs font-semibold">
                    {identity.user.displayName.slice(0, 1).toUpperCase()}
                  </div>
                  <div className="grid min-w-0 flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">
                      {identity.user.displayName}
                    </span>
                    <span className="truncate text-xs text-muted-foreground">
                      {identity.user.email}
                    </span>
                  </div>
                  <ChevronsUpDownIcon className="ml-auto" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" className="w-56">
                <DropdownMenuLabel className="font-normal">
                  <div className="grid gap-0.5 leading-tight">
                    <span className="truncate font-medium">
                      {identity.user.displayName}
                    </span>
                    <span className="truncate text-xs text-muted-foreground">
                      {identity.user.email}
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
