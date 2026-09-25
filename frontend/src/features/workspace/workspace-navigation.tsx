/** 工作台左侧模块栏和用户菜单。 */
import { useRef, useState, type ReactNode } from "react"
import {
  BellIcon,
  BotIcon,
  BrainCircuitIcon,
  GlobeIcon,
  Building2Icon,
  HeadsetIcon,
  ChevronLeftIcon,
  CodeXmlIcon,
  ContactRoundIcon,
  InboxIcon,
  LoaderCircleIcon,
  LockKeyholeIcon,
  LogOutIcon,
  HardDriveIcon,
  MonitorSmartphoneIcon,
  SearchIcon,
  SettingsIcon,
  ShieldCheckIcon,
  SlidersHorizontalIcon,
  UserRoundIcon,
  UsersRoundIcon,
  WebhookIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  updateUserWorkStatus,
  type Identity,
  type WorkStatus,
} from "@/api"
import { PagePaneGroup, PagePaneLink } from "@/components/page-split"
import { useGlobalSearch } from "@/contexts/global-search-context"
import { useUnsavedChangesContext } from "@/contexts/unsaved-changes-context"
import { agentsModulePaths } from "@/features/agents/agents-module-layout"
import { ChatRailSections } from "@/features/inbox/chat-rail"
import { WorkspaceRailToggle } from "@/features/workspace/workspace-rail"
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
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { WorkStatusDot, WorkStatusPicker } from "@/components/work-status"
import { UserAvatar } from "@/components/user-avatar"
import { cn } from "@/lib/utils"
import { resolveAppPlatform } from "@/platform/app-platform"
import { requestNotificationPermissionFromMessageMenu } from "@/platform/notifications"

/** 打开全局搜索的入口，尺寸与导航项一致；窄栏下收为图标并由浮层提示。 */
function WorkspaceSearchEntry({ collapsed }: { collapsed: boolean }) {
  const { t } = useTranslation("common")
  const globalSearch = useGlobalSearch()

  const trigger = (
    <button
      type="button"
      className={cn(
        "flex h-8 shrink-0 items-center text-sm focus-visible:ring-2 focus-visible:ring-sidebar-ring",
        collapsed
          ? "w-8 justify-center rounded-md hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
          : "w-full min-w-0 flex-1 gap-2 rounded-full bg-background/45 px-3 text-muted-foreground/65 hover:bg-background/70 hover:text-muted-foreground",
      )}
      title={collapsed ? undefined : t("actions.searchShortcut")}
      aria-label={collapsed ? t("actions.searchPlaceholder") : undefined}
      onClick={() => globalSearch?.open()}
    >
      <SearchIcon className="size-4 shrink-0" />
      {collapsed ? null : (
        <span className="min-w-0 flex-1 truncate text-left">
          {t("actions.searchPlaceholder")}
        </span>
      )}
    </button>
  )

  if (!collapsed) {
    return trigger
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{trigger}</TooltipTrigger>
      <TooltipContent side="right">{t("actions.searchShortcut")}</TooltipContent>
    </Tooltip>
  )
}

/** 模块栏导航，模块之后按群聊与单聊分节列出本人参与的聊天。 */
function WorkspaceMenu({
  identity,
  collapsed,
  railToggle,
  pendingCount,
  onInboxClick,
}: {
  identity: Identity
  collapsed: boolean
  railToggle: ReactNode
  pendingCount: number
  onInboxClick: () => void
}) {
  const { t } = useTranslation(["workspace", "inbox"])

  return (
    <nav
      // 展开时右侧留白由主内容区的内缩间隙承担，使选中块与两侧可见边界等距；窄栏下图标整列居中。
      className={cn(
        "flex min-h-0 flex-1 flex-col",
        collapsed
          ? "items-center gap-1.5 pt-1.5"
          : "items-stretch gap-0.5 pt-1 pr-0 pl-1.5",
      )}
      aria-label={t("navigationGroup")}
    >
      {collapsed ? (
        <WorkspaceSearchEntry collapsed />
      ) : (
        <div className={cn("mb-4 flex items-center gap-1", railToggle && "pr-3")}>
          <WorkspaceSearchEntry collapsed={false} />
          {railToggle}
        </div>
      )}
      {/* 展开时导航项右侧额外留白，选中块与主内容卡片边缘拉开距离，搜索框保持原有宽度；聊天较多时模块之下整体滚动。 */}
      <div
        className={cn(
          "flex min-h-0 flex-1 flex-col overflow-y-auto",
          collapsed ? "items-center gap-1.5" : "items-stretch gap-0.5 pr-3",
        )}
      >
        <PagePaneLink
          to="/inbox"
          icon={InboxIcon}
          collapsed={collapsed}
          count={pendingCount}
          countTone="neutral"
          countLabel={t("inbox:pendingCount", { count: pendingCount })}
          onClick={onInboxClick}
        >
          {t("inbox")}
        </PagePaneLink>
        <PagePaneLink
          to="/ai-employees"
          activePath={agentsModulePaths}
          icon={BotIcon}
          collapsed={collapsed}
        >
          {t("agents")}
        </PagePaneLink>
        <PagePaneLink
          to="/contacts/employees"
          activePath="/contacts"
          icon={ContactRoundIcon}
          collapsed={collapsed}
        >
          {t("contacts")}
        </PagePaneLink>
        <div className={cn("flex flex-col", collapsed ? "items-center gap-1.5" : "mt-3 gap-3")}>
          <ChatRailSections identity={identity} collapsed={collapsed} />
        </div>
      </div>
    </nav>
  )
}

/** 设置导航，进入设置后替换模块栏内容。 */
function WorkspaceSettingsMenu({
  appHref,
  collapsed,
  railToggle,
}: {
  appHref: string
  collapsed: boolean
  railToggle: ReactNode
}) {
  const { t } = useTranslation("settings")

  const backToApp = (
    <PagePaneLink
      to={appHref}
      icon={ChevronLeftIcon}
      collapsed={collapsed}
      // 左箭头字形本身内缩，展开时整行左移抵消，与下方导航项视觉左对齐。
      className={collapsed ? undefined : "-ml-1"}
    >
      {t("backToApp")}
    </PagePaneLink>
  )

  return (
    <nav
      className={cn(
        "flex min-h-0 flex-1 flex-col overflow-y-auto",
        collapsed
          ? "items-center gap-1.5 pt-1.5"
          : "items-stretch gap-0.5 pt-1 pr-0 pl-1.5",
      )}
      aria-label={t("navigationLabel")}
    >
      {railToggle ? (
        <div className="flex items-center gap-1 pr-3">
          <div className="min-w-0 flex-1">{backToApp}</div>
          {railToggle}
        </div>
      ) : (
        backToApp
      )}
      <PagePaneGroup title={t("groups.personal")} collapsed={collapsed}>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/profile"
          icon={UserRoundIcon}
        >
          {t("navigation.profile")}
        </PagePaneLink>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/security"
          icon={LockKeyholeIcon}
        >
          {t("navigation.security")}
        </PagePaneLink>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/preferences"
          icon={SlidersHorizontalIcon}
        >
          {t("navigation.preferences")}
        </PagePaneLink>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/notifications"
          icon={BellIcon}
        >
          {t("navigation.notifications")}
        </PagePaneLink>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/devices"
          icon={MonitorSmartphoneIcon}
        >
          {t("navigation.devices")}
        </PagePaneLink>
        {resolveAppPlatform() === "desktop" ? (
          <PagePaneLink
            collapsed={collapsed}
            to="/settings/local"
            icon={HardDriveIcon}
          >
            {t("navigation.local")}
          </PagePaneLink>
        ) : null}
      </PagePaneGroup>
      <PagePaneGroup title={t("groups.organization")} collapsed={collapsed}>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/general"
          icon={Building2Icon}
        >
          {t("navigation.general")}
        </PagePaneLink>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/customer-service"
          icon={HeadsetIcon}
        >
          {t("navigation.customerService")}
        </PagePaneLink>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/members"
          icon={UsersRoundIcon}
        >
          {t("navigation.members")}
        </PagePaneLink>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/roles"
          icon={ShieldCheckIcon}
        >
          {t("navigation.roles")}
        </PagePaneLink>
      </PagePaneGroup>
      {/* 集成：模型服务与联网搜索供 AI 使用，Webhooks 与开放 API 供外部系统调用 Cervi。 */}
      <PagePaneGroup title={t("groups.integrations")} collapsed={collapsed}>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/model-services"
          icon={BrainCircuitIcon}
        >
          {t("navigation.modelServices")}
        </PagePaneLink>
        <PagePaneLink
          collapsed={collapsed}
          to="/settings/web-search"
          icon={GlobeIcon}
        >
          {t("navigation.webSearch")}
        </PagePaneLink>
        <PagePaneLink collapsed={collapsed} icon={WebhookIcon}>
          {t("navigation.webhooks")}
        </PagePaneLink>
        <PagePaneLink collapsed={collapsed} icon={CodeXmlIcon}>
          {t("navigation.openApi")}
        </PagePaneLink>
      </PagePaneGroup>
    </nav>
  )
}

/** 渲染模块栏和用户菜单。 */
export function WorkspaceNavigation({
  identity,
  inSettings,
  appHref,
  collapsed,
  inlineRailToggle,
  pendingCount,
  onToggleRail,
  onLogout,
  loggingOut,
}: {
  identity: Identity
  inSettings: boolean
  appHref: string
  collapsed: boolean
  inlineRailToggle: boolean
  pendingCount: number
  onToggleRail: () => void
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
  // 展开态的收起开关按工作台布局放在一级栏顶部行的右侧或标题栏操作行。
  const railToggle =
    !collapsed && inlineRailToggle ? (
      <WorkspaceRailToggle collapsed={false} onToggle={onToggleRail} />
    ) : null

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
      {collapsed ? (
        <div className="flex shrink-0 justify-center pb-1.5">
          <WorkspaceRailToggle collapsed onToggle={onToggleRail} />
        </div>
      ) : null}
      {inSettings ? (
        <WorkspaceSettingsMenu
          appHref={appHref}
          collapsed={collapsed}
          railToggle={railToggle}
        />
      ) : (
        <WorkspaceMenu
          identity={identity}
          collapsed={collapsed}
          railToggle={railToggle}
          pendingCount={pendingCount}
          onInboxClick={requestMessageNotificationPermission}
        />
      )}
      {inSettings ? null : (
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
                      {identity.user.email}
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
                      handlesCustomers={identity.user.handlesCustomers}
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
      {showAppVersion && !collapsed ? (
        <span className="pt-1 pr-0 pb-2.5 pl-4 text-xs text-muted-foreground/70">
          {t("appVersion", { version: __APP_VERSION__ })}
        </span>
      ) : null}
    </aside>
  )
}
