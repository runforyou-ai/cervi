import { useEffect, useState } from "react"
import {
  ChevronRightIcon,
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

import type { Principal } from "@/api/identity"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
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
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar"
import { cn } from "@/lib/utils"

function ComingSoonChannel({ title }: { title: string }) {
  const { t } = useTranslation("workspace")

  return (
    <SidebarMenuSubItem>
      <SidebarMenuSubButton
        asChild
        aria-disabled="true"
      >
        <span className="justify-between" title={t("comingSoon")}>
          <span>{title}</span>
          <span className="rounded-sm bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
            {t("comingSoon")}
          </span>
        </span>
      </SidebarMenuSubButton>
    </SidebarMenuSubItem>
  )
}

function WorkspaceMenu() {
  const { t } = useTranslation("workspace")
  const location = useLocation()
  const {
    state,
    isNarrowViewport,
    setOpenNarrowViewport,
  } = useSidebar()
  const channelsActive = location.pathname.startsWith("/channels/")
  const contactsActive = location.pathname.startsWith("/contacts")
  const inboxActive = location.pathname === "/inbox"
  const [channelsOpen, setChannelsOpen] = useState(channelsActive)

  useEffect(() => {
    if (channelsActive) {
      setChannelsOpen(true)
    }
  }, [channelsActive, location.pathname])

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

          <Collapsible asChild open={channelsOpen} onOpenChange={setChannelsOpen}>
            <SidebarMenuItem>
              {state === "collapsed" && !isNarrowViewport ? (
                <SidebarMenuButton
                  asChild
                  isActive={channelsActive}
                  tooltip={t("messageChannels")}
                >
                  <NavLink to="/channels/website">
                    <MessagesSquareIcon />
                    <span>{t("messageChannels")}</span>
                  </NavLink>
                </SidebarMenuButton>
              ) : (
                <CollapsibleTrigger asChild>
                  <SidebarMenuButton
                    isActive={channelsActive && !channelsOpen}
                    aria-label={t("toggleMessageChannels")}
                  >
                    <MessagesSquareIcon />
                    <span>{t("messageChannels")}</span>
                    <ChevronRightIcon
                      className={cn(
                        "ml-auto transition-transform duration-200",
                        channelsOpen && "rotate-90"
                      )}
                    />
                  </SidebarMenuButton>
                </CollapsibleTrigger>
              )}

              <CollapsibleContent className="overflow-hidden data-[state=closed]:animate-[collapsible-up_160ms_ease-in] data-[state=open]:animate-[collapsible-down_180ms_ease-out]">
                <SidebarMenuSub>
                  <SidebarMenuSubItem>
                    <SidebarMenuSubButton
                      asChild
                      isActive={location.pathname.startsWith(
                        "/channels/website"
                      )}
                    >
                      <NavLink
                        to="/channels/website"
                        onClick={closeNarrowNavigation}
                      >
                        <span>{t("website")}</span>
                      </NavLink>
                    </SidebarMenuSubButton>
                  </SidebarMenuSubItem>
                  <ComingSoonChannel title={t("telegram")} />
                  <ComingSoonChannel title={t("wechatOfficialAccount")} />
                </SidebarMenuSub>
              </CollapsibleContent>
            </SidebarMenuItem>
          </Collapsible>
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

export function WorkspaceNavigation({
  principal,
  onLogout,
  loggingOut,
}: {
  principal: Principal
  onLogout: () => void
  loggingOut: boolean
}) {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const { setOpenNarrowViewport } = useSidebar()
  const [userMenuOpen, setUserMenuOpen] = useState(false)

  function openSettings() {
    navigate("/settings")
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
                  {principal.organization.name}
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
                  tooltip={userMenuOpen ? undefined : principal.user.displayName}
                  aria-label={t("openUserMenu", {
                    name: principal.user.displayName,
                  })}
                >
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-xs font-semibold">
                    {principal.user.displayName.slice(0, 1).toUpperCase()}
                  </div>
                  <div className="grid min-w-0 flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">
                      {principal.user.displayName}
                    </span>
                    <span className="truncate text-xs text-muted-foreground">
                      {principal.user.email}
                    </span>
                  </div>
                  <ChevronsUpDownIcon className="ml-auto" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" className="w-56">
                <DropdownMenuLabel className="font-normal">
                  <div className="grid gap-0.5 leading-tight">
                    <span className="truncate font-medium">
                      {principal.user.displayName}
                    </span>
                    <span className="truncate text-xs text-muted-foreground">
                      {principal.user.email}
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
