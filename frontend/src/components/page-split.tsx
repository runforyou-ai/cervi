/** 页面内容区的左右分栏。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { NavLink } from "react-router"

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
          "min-h-0 shrink-0 flex-col overflow-hidden border-r",
          paneOnNarrowClass[paneOnNarrow],
          paneWidthClass[paneWidth],
          paneVariant === "nav" && "bg-sidebar text-sidebar-foreground",
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
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <ScrollArea className="min-h-0 flex-1">
      <nav className="flex flex-col gap-1 p-3" aria-label={label}>
        {children}
      </nav>
    </ScrollArea>
  )
}

/** 分栏左栏导航项。 */
export function PagePaneLink({
  to,
  comingSoon = false,
  children,
}: {
  to?: string
  comingSoon?: boolean
  children: ReactNode
}) {
  const { t } = useTranslation("common")
  const className =
    "flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors"

  if (comingSoon || !to) {
    return (
      <span
        className={cn(className, "cursor-default text-muted-foreground")}
        aria-disabled="true"
        title={comingSoon ? t("comingSoon") : undefined}
      >
        <span className="min-w-0 flex-1 truncate">{children}</span>
        {comingSoon ? (
          <span className="rounded-sm bg-sidebar-accent px-1.5 py-0.5 text-[10px] text-muted-foreground">
            {t("comingSoon")}
          </span>
        ) : null}
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
          isActive &&
            "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
        )
      }
    >
      <span className="min-w-0 flex-1 truncate">{children}</span>
    </NavLink>
  )
}
