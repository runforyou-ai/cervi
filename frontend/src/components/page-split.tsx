/** 页面内容区的左右分栏。 */
import type { ReactNode } from "react"
import type { LucideIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { NavLink, useLocation } from "react-router"

import { StatusBadge } from "@/components/status-badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

const paneWidthClass = {
  sm: "md:w-48",
  md: "md:w-56",
  lg: "md:w-72",
  /* 消息页中栏：64px 范围纵栏 + 312px 会话列表（列表宽与 helmdesk 中栏一致）。 */
  inbox: "md:w-94",
  /* 消息页中栏收起范围纵栏后只保留会话列表。 */
  inboxCollapsed: "md:w-78",
} as const

const paneOnNarrowClass = {
  hide: "hidden md:flex",
  fill: "flex w-full",
} as const

export type PageSplitPaneWidth = keyof typeof paneWidthClass
export type PageSplitPaneOnNarrow = keyof typeof paneOnNarrowClass

/** 按档位宽度分割左栏和主区。 */
export function PageSplit({
  pane,
  paneWidth = "md",
  paneOnNarrow = "hide",
  paneVariant = "plain",
  paneClassName,
  mainClassName,
  className,
  children,
}: {
  pane: ReactNode
  paneWidth?: PageSplitPaneWidth
  paneOnNarrow?: PageSplitPaneOnNarrow
  paneVariant?: "plain" | "nav"
  paneClassName?: string
  mainClassName?: string
  className?: string
  children: ReactNode
}) {
  return (
    <div
      data-slot="page-split"
      className={cn("flex min-h-0 w-full flex-1 overflow-hidden", className)}
    >
      <aside
        data-slot="page-split-pane"
        className={cn(
          "min-h-0 shrink-0 flex-col overflow-hidden border-r select-none",
          paneOnNarrowClass[paneOnNarrow],
          paneWidthClass[paneWidth],
          paneVariant === "nav" &&
            "border-sidebar-border bg-sidebar-secondary text-sidebar-foreground",
          paneClassName,
        )}
      >
        {pane}
      </aside>
      <div
        data-slot="page-split-main"
        className={cn(
          "flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden",
          mainClassName,
        )}
      >
        {children}
      </div>
    </div>
  )
}

/** 分栏左栏导航列表。 */
export function PagePaneNav({
  label,
  title,
  action,
  children,
}: {
  label: string
  title?: string
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {title ? (
        <div className="flex shrink-0 items-center gap-2 px-3.5 pt-4 pb-1.5">
          <h2 className="min-w-0 flex-1 truncate text-lg font-semibold tracking-tight">
            {title}
          </h2>
          {action}
        </div>
      ) : null}
      <ScrollArea className="min-h-0 flex-1">
        <nav className="flex flex-col gap-0.5 p-2.5" aria-label={label}>
          {children}
        </nav>
      </ScrollArea>
    </div>
  )
}

/** 分栏左栏导航项；activePath 按公共路径前缀保持整组页面的选中态，窄栏下只显示图标并由浮层提示名称。 */
export function PagePaneLink({
  to,
  activePath,
  icon: Icon,
  collapsed,
  children,
}: {
  to?: string
  activePath?: string
  icon?: LucideIcon
  collapsed?: boolean
  children: ReactNode
}) {
  const { t } = useTranslation("common")
  const { pathname } = useLocation()
  const prefixActive =
    activePath !== undefined &&
    (pathname === activePath || pathname.startsWith(`${activePath}/`))
  // 选中态按当前路径判定，className 保持字符串形式，供窄栏下的浮层触发器合并。
  const active =
    prefixActive ||
    (to !== undefined && (pathname === to || pathname.startsWith(`${to}/`)))
  const className = cn(
    "flex h-8 shrink-0 items-center rounded-md text-left text-sm transition-colors",
    collapsed ? "w-8 justify-center" : "w-full gap-2 px-2.5",
  )
  const label = collapsed ? (
    <span className="sr-only">{children}</span>
  ) : (
    <span className="min-w-0 flex-1 truncate">{children}</span>
  )

  const item = to ? (
    <NavLink
      to={to}
      className={cn(
        className,
        "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
        active && "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
      )}
    >
      {Icon ? <Icon className="size-4 shrink-0" /> : null}
      {label}
    </NavLink>
  ) : (
    <span
      className={cn(className, "cursor-default text-muted-foreground")}
      aria-disabled="true"
      title={collapsed ? undefined : t("comingSoon")}
    >
      {Icon ? <Icon className="size-4 shrink-0" /> : null}
      {label}
      {collapsed ? null : (
        <StatusBadge variant="muted">{t("comingSoon")}</StatusBadge>
      )}
    </span>
  )

  if (!collapsed) {
    return item
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{item}</TooltipTrigger>
      <TooltipContent side="right">
        {children}
        {to ? null : ` · ${t("comingSoon")}`}
      </TooltipContent>
    </Tooltip>
  )
}

/** 分栏左栏导航分组，分组标题下是同组导航项；窄栏下标题由分隔线代替。 */
export function PagePaneGroup({
  title,
  collapsed,
  children,
}: {
  title: string
  collapsed?: boolean
  children: ReactNode
}) {
  if (collapsed) {
    return (
      <div className="mt-1.5 flex flex-col items-center gap-1.5 border-t border-sidebar-border pt-1.5">
        {children}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-0.5">
      <span className="px-2.5 pt-2.5 pb-0.5 text-[11px] font-semibold tracking-[0.12em] text-muted-foreground/70 uppercase">
        {title}
      </span>
      {children}
    </div>
  )
}
