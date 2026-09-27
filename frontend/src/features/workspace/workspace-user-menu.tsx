/** 工作台左下角的用户菜单：工作状态、设置入口、切换或创建工作区与退出登录。 */
import { useRef, useState } from "react"
import {
  CheckIcon,
  LayoutGridIcon,
  LoaderCircleIcon,
  LogOutIcon,
  PlusIcon,
  SettingsIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import type { Identity } from "@/api"
import { useUnsavedChangesContext } from "@/contexts/unsaved-changes-context"
import { useWorkspaceScope } from "@/contexts/workspace-scope-context"
import { useWorkStatusChange } from "@/hooks/use-work-status-change"
import { enterWorkspace, navigateToHashPath } from "@/lib/workspace-route"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { WorkStatusDot, WorkStatusPicker } from "@/components/work-status"
import { UserAvatar } from "@/components/user-avatar"
import { cn } from "@/lib/utils"

/** 展示当前成员头像与工作区，展开后切换工作状态、进入设置、切换工作区或退出登录。 */
export function WorkspaceUserMenu({
  identity,
  collapsed,
  loggingOut,
  onLogout,
}: {
  identity: Identity
  collapsed: boolean
  loggingOut: boolean
  onLogout: () => void
}) {
  const { t } = useTranslation(["workspace", "account"])
  const workspaceScope = useWorkspaceScope()
  const navigate = useNavigate()
  const unsavedChanges = useUnsavedChangesContext()
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const workStatus = useWorkStatusChange(identity.user.workStatus)
  const userMenuTriggerRef = useRef<HTMLButtonElement>(null)
  const skipUserMenuFocusRestoreRef = useRef(false)

  /** 从用户菜单进入页面，并清除头像触发器的选中效果。 */
  function navigateFromUserMenu(path: string) {
    skipUserMenuFocusRestoreRef.current = true
    navigate(path)
  }

  return (
    <div
      className={cn("shrink-0 pt-1 pb-2.5", collapsed ? "px-0" : "pr-0 pl-1.5")}
    >
      <DropdownMenu open={userMenuOpen} onOpenChange={setUserMenuOpen}>
        <DropdownMenuTrigger asChild>
          <button
            ref={userMenuTriggerRef}
            type="button"
            className={cn(
              "flex w-full items-center rounded-md text-left outline-none hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring",
              collapsed
                ? "justify-center py-1"
                : "gap-2.5 px-2.5 py-1.5",
            )}
            aria-label={t("openUserMenu", {
              name: identity.user.displayName,
            })}
          >
            <span className="relative size-8 shrink-0">
              <UserAvatar
                user={identity.user}
                className="size-full rounded-lg"
              />
              <WorkStatusDot
                status={identity.user.workStatus}
                className="absolute -right-0.5 -bottom-0.5 ring-2 ring-sidebar"
              />
            </span>
            {collapsed ? null : (
              <span className="grid min-w-0 flex-1 gap-0.5 leading-tight">
                <span className="truncate text-sm font-medium">
                  {identity.user.displayName}
                </span>
                <span className="truncate text-xs text-muted-foreground">
                  {workspaceScope.current.name}
                </span>
              </span>
            )}
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          side="top"
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
          <DropdownMenuLabel className="p-2 font-normal">
            <div className="flex items-center gap-2.5">
              <div className="relative size-9 shrink-0">
                <UserAvatar
                  user={identity.user}
                  className="size-9 rounded-lg"
                />
                <WorkStatusDot
                  status={identity.user.workStatus}
                  className="absolute -right-0.5 -bottom-0.5 ring-2 ring-popover"
                />
              </div>
              <div className="grid min-w-0 flex-1 translate-y-0.5 gap-0.5 leading-tight">
                <span className="truncate text-base font-medium">
                  {identity.user.displayName}
                </span>
                <WorkStatusPicker
                  status={identity.user.workStatus}
                  handlesServiceRequests={identity.user.handlesServiceRequests}
                  onChange={(next) => void workStatus.change(next)}
                />
              </div>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onSelect={() => navigateFromUserMenu("/settings/profile")}
          >
            <SettingsIcon />
            {t("settings")}
          </DropdownMenuItem>
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>
              <LayoutGridIcon />
              {t("account:switchWorkspace")}
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent className="w-56">
              {workspaceScope.workspaces.map((workspace) => (
                <DropdownMenuItem
                  key={workspace.id}
                  onSelect={async () => {
                    if (workspace.id === workspaceScope.current.id) return
                    setUserMenuOpen(false)
                    if (unsavedChanges && !(await unsavedChanges.confirmDiscard())) return
                    enterWorkspace(workspace.slug)
                  }}
                >
                  <span className="min-w-0 flex-1 truncate">{workspace.name}</span>
                  {workspace.id === workspaceScope.current.id ? <CheckIcon /> : null}
                </DropdownMenuItem>
              ))}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onSelect={async () => {
                  setUserMenuOpen(false)
                  if (unsavedChanges && !(await unsavedChanges.confirmDiscard())) return
                  navigateToHashPath("/workspaces/new?from=workspace")
                }}
              >
                <PlusIcon />
                {t("account:create")}
              </DropdownMenuItem>
            </DropdownMenuSubContent>
          </DropdownMenuSub>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            destructive
            disabled={loggingOut}
            onSelect={async () => {
              setUserMenuOpen(false)
              if (unsavedChanges && !(await unsavedChanges.confirmDiscard())) return
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
  )
}
