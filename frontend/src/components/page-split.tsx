/** 页面内容区的左右分栏。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { NavLink, useLocation } from "react-router"

import { StatusBadge } from "@/components/status-badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { cn } from "@/lib/utils"

const paneWidthClass = {
  sm: "md:w-52",
  md: "md:w-60",
  lg: "md:w-80",
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
        <div className="flex shrink-0 items-center gap-2 px-4 pt-5 pb-2">
          <h2 className="min-w-0 flex-1 truncate text-xl font-semibold tracking-tight">
            {title}
          </h2>
          {action}
        </div>
      ) : null}
      <ScrollArea className="min-h-0 flex-1">
        <nav className="flex flex-col gap-1 p-3" aria-label={label}>
          {children}
        </nav>
      </ScrollArea>
    </div>
  )
}

/** 分栏左栏导航项；activePath 按公共路径前缀保持整组页面的选中态。 */
export function PagePaneLink({
  to,
  activePath,
  children,
}: {
  to?: string
  activePath?: string
  children: ReactNode
}) {
  const { t } = useTranslation("common")
  const { pathname } = useLocation()
  const prefixActive =
    activePath !== undefined &&
    (pathname === activePath || pathname.startsWith(`${activePath}/`))
  const className =
    "flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors"

  if (!to) {
    return (
      <span
        className={cn(className, "cursor-default text-muted-foreground")}
        aria-disabled="true"
        title={t("comingSoon")}
      >
        <span className="min-w-0 flex-1 truncate">{children}</span>
        <StatusBadge variant="muted">{t("comingSoon")}</StatusBadge>
      </span>
    )
  }

  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(
          className,
          "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
          (isActive || prefixActive) &&
            "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
        )
      }
    >
      <span className="min-w-0 flex-1 truncate">{children}</span>
    </NavLink>
  )
}
