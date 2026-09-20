/** 工作台左侧模块栏和用户菜单。 */
import { useRef, useState } from "react"
import {
  BrainCircuitIcon,
  Building2Icon,
  CheckIcon,
  ChevronLeftIcon,
  CodeXmlIcon,
  ContactRoundIcon,
  InboxIcon,
  LibraryIcon,
  LoaderCircleIcon,
  LockKeyholeIcon,
  LogOutIcon,
  MessagesSquareIcon,
  MonitorSmartphoneIcon,
  PlugIcon,
  SettingsIcon,
  ShieldCheckIcon,
  SlidersHorizontalIcon,
  UserRoundIcon,
  WebhookIcon,
  type LucideIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { NavLink, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  updateUserWorkStatus,
  type Identity,
  type WorkStatus,
} from "@/api"
import { PagePaneGroup, PagePaneLink } from "@/components/page-split"
import { useUnsavedChangesContext } from "@/contexts/unsaved-changes-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
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
  workStatusTextClass,
  workStatusTintClass,
} from "@/components/work-status"
import { UserAvatar } from "@/components/user-avatar"
import { cn } from "@/lib/utils"
import { resolveAppPlatform } from "@/platform/app-platform"
import { requestNotificationPermissionFromMessageMenu } from "@/platform/notifications"

/** 模块栏导航项。 */
function WorkspaceRailItem({
  to,
  icon: Icon,
  label,
  active,
  onClick,
  className,
}: {
  to: string
  icon: LucideIcon
  label: string
  active: boolean
  onClick?: () => void
  className?: string
}) {
  return (
    <NavLink
      to={to}
      onClick={onClick}
      className={cn(
        "flex h-8 w-full items-center gap-2 rounded-md px-2.5 text-sm",
        "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
        "focus-visible:ring-2 focus-visible:ring-sidebar-ring",
        active &&
          "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
        className,
      )}
    >
      <Icon className="size-4 shrink-0" />
      <span className="min-w-0 flex-1 truncate">{label}</span>
    </NavLink>
  )
}

/** 模块栏导航。 */
function WorkspaceMenu({
  onInboxClick,
}: {
  onInboxClick: () => void
}) {
  const { t } = useTranslation("workspace")
  const location = useLocation()

  return (
    <nav
      // 右侧留白由主内容区的内缩间隙承担，使选中块与两侧可见边界等距。
      className="flex flex-1 flex-col items-stretch gap-0.5 pt-1 pr-0 pl-1.5"
      aria-label={t("navigationGroup")}
    >
      <WorkspaceRailItem
        to="/inbox"
        icon={InboxIcon}
        label={t("inbox")}
        active={location.pathname === "/inbox"}
        onClick={onInboxClick}
      />
      <WorkspaceRailItem
        to="/contacts/employees"
        icon={ContactRoundIcon}
        label={t("contacts")}
        active={location.pathname.startsWith("/contacts")}
      />
      <WorkspaceRailItem
        to="/knowledge-bases"
        icon={LibraryIcon}
        label={t("knowledgeBases")}
        active={location.pathname.startsWith("/knowledge-bases")}
      />
    </nav>
  )
}

/** 设置导航，进入设置后替换模块栏内容。 */
function WorkspaceSettingsMenu({ appHref }: { appHref: string }) {
  const { t } = useTranslation("settings")

  return (
    <nav
      className="flex min-h-0 flex-1 flex-col items-stretch gap-0.5 overflow-y-auto pt-1 pr-0 pl-1.5"
      aria-label={t("navigationLabel")}
    >
      <WorkspaceRailItem
        to={appHref}
        icon={ChevronLeftIcon}
        label={t("backToApp")}
        active={false}
        // 左箭头字形本身内缩，整行左移抵消，与下方导航项视觉左对齐。
        className="-ml-1"
      />
      <PagePaneGroup title={t("groups.personal")}>
        <PagePaneLink to="/settings/profile" icon={UserRoundIcon}>
          {t("navigation.profile")}
        </PagePaneLink>
        <PagePaneLink to="/settings/security" icon={LockKeyholeIcon}>
          {t("navigation.security")}
        </PagePaneLink>
        <PagePaneLink to="/settings/preferences" icon={SlidersHorizontalIcon}>
          {t("navigation.preferences")}
        </PagePaneLink>
        <PagePaneLink to="/settings/devices" icon={MonitorSmartphoneIcon}>
          {t("navigation.devices")}
        </PagePaneLink>
      </PagePaneGroup>
      <PagePaneGroup title={t("groups.organization")}>
        <PagePaneLink to="/settings/general" icon={Building2Icon}>
          {t("navigation.general")}
        </PagePaneLink>
        <PagePaneLink
          to="/settings/roles"
          activePath="/settings/roles"
          icon={ShieldCheckIcon}
        >
          {t("navigation.roles")}
        </PagePaneLink>
        <PagePaneLink to="/settings/channels" icon={MessagesSquareIcon}>
          {t("navigation.channels")}
        </PagePaneLink>
        <PagePaneLink
          to="/settings/model-services/chat"
          activePath="/settings/model-services"
          icon={BrainCircuitIcon}
        >
          {t("navigation.modelServices")}
        </PagePaneLink>
        <PagePaneLink to="/settings/mcp-servers" icon={PlugIcon}>
          {t("navigation.mcpServers")}
        </PagePaneLink>
        <PagePaneLink icon={WebhookIcon}>{t("navigation.webhooks")}</PagePaneLink>
        <PagePaneLink icon={CodeXmlIcon}>{t("navigation.openApi")}</PagePaneLink>
      </PagePaneGroup>
    </nav>
  )
}

/** 用当前工作状态文字触发的状态选择菜单。 */
function WorkStatusPicker({
  status,
  onChange,
}: {
  status: WorkStatus
  onChange: (workStatus: WorkStatus) => void
}) {
  const { t } = useTranslation("workspace")
  const { t: tCommon } = useTranslation("common")

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className={cn(
            "w-fit max-w-full truncate rounded-full px-2.5 text-left text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring",
            workStatusTextClass(status),
            workStatusTintClass(status),
          )}
          aria-label={t("workStatus")}
        >
          {workStatusLabel(status, tCommon)}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="start" className="w-36">
        {selectableWorkStatuses.map((workStatus) => (
          <DropdownMenuItem
            key={workStatus}
            onSelect={(event) => {
              event.preventDefault()
              onChange(workStatus)
            }}
          >
            <WorkStatusDot status={workStatus} />
            <span className="flex-1">{workStatusLabel(workStatus, tCommon)}</span>
            {workStatus === status ? (
              <CheckIcon className="text-primary" />
            ) : null}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** 渲染模块栏和用户菜单。 */
export function WorkspaceNavigation({
  identity,
  inSettings,
  appHref,
  onLogout,
  loggingOut,
}: {
  identity: Identity
  inSettings: boolean
  appHref: string
  onLogout: () => void
  loggingOut: boolean
}) {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const unsavedChanges = useUnsavedChangesContext()
  const invalidate = useResourceInvalidator()
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const changingWorkStatusRef = useRef(false)
  const userMenuTriggerRef = useRef<HTMLButtonElement>(null)
  const skipUserMenuFocusRestoreRef = useRef(false)
  const showAppVersion = inSettings && resolveAppPlatform() === "desktop"

  /** 从用户菜单进入页面，并清除头像触发器的选中效果。 */
  function navigateFromUserMenu(path: string) {
    skipUserMenuFocusRestoreRef.current = true
    navigate(path)
  }

  /** 点击消息菜单时申请本设备通知权限。 */
  function requestMessageNotificationPermission() {
    if (!identity.user.messageNotificationsEnabled) {
      return
    }

    void requestNotificationPermissionFromMessageMenu({
      organizationId: identity.user.organizationId,
      userId: identity.user.id,
    })
      .catch((error) => {
        console.warn("从消息菜单申请通知权限失败", error)
      })
  }

  /** 保存工作状态并刷新当前身份。 */
  async function changeWorkStatus(workStatus: WorkStatus) {
    if (
      workStatus === identity.user.workStatus ||
      changingWorkStatusRef.current
    ) {
      return
    }

    changingWorkStatusRef.current = true
    try {
      await updateUserWorkStatus({ workStatus })
      void invalidate(resourceKeys.identity())
    } catch (error) {
      if (!recoverSession(error, navigate)) {
        console.warn("切换工作状态失败", error)
        toast.error(t("workStatusUpdateError"))
      }
    } finally {
      changingWorkStatusRef.current = false
    }
  }

  return (
    <aside className="cervi-workspace-rail flex h-full shrink-0 flex-col text-sidebar-foreground">
      {inSettings ? (
        <WorkspaceSettingsMenu appHref={appHref} />
      ) : (
        <WorkspaceMenu onInboxClick={requestMessageNotificationPermission} />
      )}
      {inSettings ? null : (
        <div className="pt-1 pr-0 pb-2.5 pl-1.5">
          <DropdownMenu open={userMenuOpen} onOpenChange={setUserMenuOpen}>
            <DropdownMenuTrigger asChild>
              <button
                ref={userMenuTriggerRef}
                type="button"
                className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left outline-none hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring"
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
                <span className="grid min-w-0 flex-1 gap-0.5 leading-tight">
                  <span className="truncate text-sm font-medium">
                    {identity.user.displayName}
                  </span>
                  <span className="truncate text-xs text-muted-foreground">
                    {identity.user.email}
                  </span>
                </span>
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
                      className="size-9 rounded-lg text-sm"
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
                      onChange={changeWorkStatus}
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
      )}
      {showAppVersion ? (
        <span className="pt-1 pr-0 pb-2.5 pl-4 text-[11px] text-muted-foreground/70">
          {t("appVersion", { version: __APP_VERSION__ })}
        </span>
      ) : null}
    </aside>
  )
}
