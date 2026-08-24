/** 工作台左侧模块轨和用户菜单。 */
import { useRef, useState } from "react"
import {
  CheckIcon,
  ContactRoundIcon,
  InboxIcon,
  LoaderCircleIcon,
  LogOutIcon,
  PlugIcon,
  SettingsIcon,
  type LucideIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { NavLink, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  updateUserWorkStatus,
  type Identity,
  type CurrentUser,
  type WorkStatus,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  selectableWorkStatuses,
  WorkStatusDot,
  workStatusLabel,
} from "@/features/users/work-status"
import { UserAvatar } from "@/features/users/user-avatar"
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
        "my-0.5 flex h-14 w-full flex-col items-center justify-center gap-1 rounded-md px-1 text-[11px] leading-tight",
        "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
        "focus-visible:ring-2 focus-visible:ring-sidebar-ring",
        active &&
          "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
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
        to="/contacts/employees"
        icon={ContactRoundIcon}
        label={t("contacts")}
        active={location.pathname.startsWith("/contacts")}
      />
      <WorkspaceRailItem
        to="/integrations/channels"
        icon={PlugIcon}
        label={t("integrations")}
        active={location.pathname.startsWith("/integrations")}
      />
      <div className="mt-auto pb-1">
        <WorkspaceRailItem
          to="/settings/organization"
          icon={SettingsIcon}
          label={t("systemSettings")}
          active={location.pathname.startsWith("/settings")}
        />
      </div>
    </nav>
  )
}

/** 渲染模块轨和用户菜单。 */
export function WorkspaceNavigation({
  identity,
  onUserUpdated,
  onLogout,
  loggingOut,
}: {
  identity: Identity
  onUserUpdated: (user: CurrentUser) => void
  onLogout: () => void
  loggingOut: boolean
}) {
  const { t } = useTranslation("workspace")
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const changingWorkStatusRef = useRef(false)
  const userMenuTriggerRef = useRef<HTMLButtonElement>(null)
  const skipUserMenuFocusRestoreRef = useRef(false)

  /** 从用户菜单进入页面，并清除头像触发器的选中效果。 */
  function navigateFromUserMenu(path: string) {
    skipUserMenuFocusRestoreRef.current = true
    navigate(path)
  }

  /** 立即保存工作状态，并在失败时恢复原状态。 */
  async function changeWorkStatus(workStatus: WorkStatus) {
    if (
      workStatus === identity.user.workStatus ||
      changingWorkStatusRef.current
    ) {
      return
    }

    const previous = identity.user
    changingWorkStatusRef.current = true
    onUserUpdated({ ...previous, workStatus })
    try {
      const updated = await updateUserWorkStatus({ workStatus })
      onUserUpdated(updated)
      console.info("工作状态已切换", { work_status: workStatus })
    } catch (error) {
      onUserUpdated(previous)
      if (!recoverSession(error, navigate)) {
        console.warn("切换工作状态失败", error)
        toast.error(t("workStatusUpdateError"))
      }
    } finally {
      changingWorkStatusRef.current = false
    }
  }

  return (
    <aside className="cervi-workspace-rail flex h-full w-[76px] shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
      <div className="flex justify-center px-3 pt-2.5 pb-1.5">
        <DropdownMenu open={userMenuOpen} onOpenChange={setUserMenuOpen}>
          <DropdownMenuTrigger asChild>
            <button
              ref={userMenuTriggerRef}
              type="button"
              className="relative flex size-10 items-center justify-center rounded-lg bg-sidebar-primary text-sm font-semibold text-sidebar-primary-foreground outline-none hover:opacity-90 focus-visible:ring-2 focus-visible:ring-sidebar-ring"
              aria-label={t("openUserMenu", {
                name: identity.user.displayName,
              })}
            >
              <UserAvatar user={identity.user} className="size-full rounded-lg" />
              <WorkStatusDot
                status={identity.user.workStatus}
                className="absolute -right-0.5 -bottom-0.5 ring-2 ring-sidebar"
              />
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
            <DropdownMenuItem
              className="p-2"
              onSelect={() => navigateFromUserMenu("/account/profile")}
            >
              <div className="flex items-center gap-3">
                <div className="relative size-10 shrink-0">
                  <UserAvatar
                    user={identity.user}
                    className="size-10 rounded-lg text-sm"
                  />
                  <WorkStatusDot
                    status={identity.user.workStatus}
                    className="absolute -right-0.5 -bottom-0.5 ring-2 ring-popover"
                  />
                </div>
                <div className="grid min-w-0 gap-1 leading-tight">
                  <span className="truncate font-medium">
                    {identity.user.displayName}
                  </span>
                  <span className="truncate text-xs text-muted-foreground">
                    {identity.user.email}
                  </span>
                </div>
              </div>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuLabel className="text-sm text-muted-foreground">
              {t("workStatus")}
            </DropdownMenuLabel>
            {selectableWorkStatuses.map((workStatus) => {
              const selected = identity.user.workStatus === workStatus
              return (
                <DropdownMenuItem
                  key={workStatus}
                  className="text-xs"
                  onSelect={(event) => {
                    event.preventDefault()
                    void changeWorkStatus(workStatus)
                  }}
                >
                  <WorkStatusDot status={workStatus} className="size-2" />
                  <span className="flex-1">
                    {workStatusLabel(workStatus, tCommon)}
                  </span>
                  {selected ? <CheckIcon className="text-primary" /> : null}
                </DropdownMenuItem>
              )
            })}
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
