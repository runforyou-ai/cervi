/** 工作台左侧模块轨和用户菜单。 */
import { useRef, useState } from "react"
import {
  ContactRoundIcon,
  InboxIcon,
  LoaderCircleIcon,
  LogOutIcon,
  MessagesSquareIcon,
  SettingsIcon,
  UserRoundIcon,
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

/** 模块轨导航项。 */
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
        "focus-visible:ring-2 focus-visible:ring-sidebar-ring",
        active
          ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
          : "hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
      )}
    >
      <Icon className="size-5" />
      <span className="line-clamp-2 text-center break-words">{label}</span>
    </NavLink>
  )
}

/** 模块轨导航。 */
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

/** 渲染模块轨和用户菜单。 */
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
  const userMenuTriggerRef = useRef<HTMLButtonElement>(null)
  const skipUserMenuFocusRestoreRef = useRef(false)

  /** 从用户菜单进入页面，并清除头像触发器的选中效果。 */
  function navigateFromUserMenu(path: string) {
    skipUserMenuFocusRestoreRef.current = true
    navigate(path)
  }

  return (
    <aside className="cervi-workspace-rail flex h-full w-[76px] shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
      <div className="flex justify-center px-3 pt-2.5 pb-1.5">
        <DropdownMenu open={userMenuOpen} onOpenChange={setUserMenuOpen}>
          <DropdownMenuTrigger asChild>
            <button
              ref={userMenuTriggerRef}
              type="button"
              className="flex size-10 items-center justify-center rounded-lg bg-sidebar-primary text-sm font-semibold text-sidebar-primary-foreground outline-none hover:opacity-90 focus-visible:ring-2 focus-visible:ring-sidebar-ring"
              aria-label={t("openUserMenu", {
                name: identity.user.displayName,
              })}
            >
              {identity.user.displayName.slice(0, 1).toUpperCase()}
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            side="right"
            align="start"
            className="w-56"
            onCloseAutoFocus={(event) => {
              if (!skipUserMenuFocusRestoreRef.current) {
                return
              }

              event.preventDefault()
              skipUserMenuFocusRestoreRef.current = false
              userMenuTriggerRef.current?.blur()
            }}
          >
            <DropdownMenuLabel className="font-normal">
              <div className="flex items-center gap-3">
                <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary text-sm font-semibold text-sidebar-primary-foreground">
                  {identity.user.displayName.slice(0, 1).toUpperCase()}
                </div>
                <div className="grid min-w-0 gap-1 leading-tight">
                  <span className="truncate font-medium">
                    {identity.user.displayName}
                  </span>
                  <span className="truncate text-xs text-muted-foreground">
                    {identity.organization.name}
                  </span>
                </div>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() => navigateFromUserMenu("/settings/profile")}
            >
              <UserRoundIcon />
              {t("profile")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={() => navigateFromUserMenu("/settings/storage")}
            >
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
