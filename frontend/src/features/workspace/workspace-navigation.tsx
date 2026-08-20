/** 工作台左侧模块轨和用户菜单。 */
import { useState } from "react"
import {
  ContactRoundIcon,
  InboxIcon,
  LoaderCircleIcon,
  LogOutIcon,
  MessagesSquareIcon,
  SettingsIcon,
  type LucideIcon,
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
import { cn } from "@/lib/utils"

/** 模块轨上的导航项。 */
function WorkspaceRailItem({
  to,
  icon: Icon,
  label,
  active,
}: {
  to: string
  icon: LucideIcon
  label: string
  active: boolean
}) {
  return (
    <NavLink
      to={to}
      className={cn(
        "flex h-16 w-full flex-col items-center justify-center gap-1 rounded-md px-1 text-[11px] leading-tight",
        "hover:bg-foreground/6 hover:text-sidebar-accent-foreground",
        "focus-visible:ring-2 focus-visible:ring-sidebar-ring",
        active && "bg-foreground/12 font-medium text-sidebar-accent-foreground",
      )}
    >
      <Icon className="size-5" />
      <span className="line-clamp-2 text-center break-words">{label}</span>
    </NavLink>
  )
}

/** 工作台主导航。 */
function WorkspaceMenu() {
  const { t } = useTranslation("workspace")
  const location = useLocation()

  return (
    <nav
      className="flex flex-1 flex-col items-stretch gap-1 px-1.5 pt-1"
      aria-label={t("navigationGroup")}
    >
      <WorkspaceRailItem
        to="/inbox"
        icon={InboxIcon}
        label={t("inbox")}
        active={location.pathname === "/inbox"}
      />
      <WorkspaceRailItem
        to="/contacts/internal"
        icon={ContactRoundIcon}
        label={t("contacts")}
        active={location.pathname.startsWith("/contacts")}
      />
      <WorkspaceRailItem
        to="/channels/website"
        icon={MessagesSquareIcon}
        label={t("channels")}
        active={location.pathname.startsWith("/channels")}
      />
    </nav>
  )
}

/** 工作台模块轨和用户菜单。 */
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
  const [userMenuOpen, setUserMenuOpen] = useState(false)

  /** 打开设置页。 */
  function openSettings() {
    console.info("打开设置")
    navigate("/settings/storage")
  }

  return (
    <aside className="flex h-full w-[72px] shrink-0 flex-col border-r bg-sidebar text-sidebar-foreground">
      <div className="flex justify-center px-3 pt-2.5 pb-1.5">
        <DropdownMenu open={userMenuOpen} onOpenChange={setUserMenuOpen}>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="flex size-10 items-center justify-center rounded-lg bg-sidebar-primary text-sm font-semibold text-sidebar-primary-foreground outline-none hover:opacity-90 focus-visible:ring-2 focus-visible:ring-sidebar-ring"
              aria-label={t("openUserMenu", {
                name: identity.user.displayName,
              })}
            >
              {identity.user.displayName.slice(0, 1).toUpperCase()}
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="start" className="w-56">
            <DropdownMenuLabel className="font-normal">
              <div className="grid gap-0.5 leading-tight">
                <span className="truncate font-medium">
                  {identity.user.displayName}
                </span>
                <span className="truncate text-xs text-muted-foreground">
                  {identity.user.email}
                </span>
                <span className="truncate text-xs text-muted-foreground">
                  {identity.organization.name}
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
      </div>
      <WorkspaceMenu />
    </aside>
  )
}
