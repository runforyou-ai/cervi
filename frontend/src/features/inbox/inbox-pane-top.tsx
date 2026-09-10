/** 消息页中栏顶部操作行。 */
import { PanelLeftIcon, PlusIcon, SearchIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

/** 顶部操作行：收纳范围栏、搜索占位和发起会话菜单。 */
export function InboxPaneTop({
  railCollapsed,
  onRailToggle,
  onCreateGroup,
  onCreateAgent,
}: {
  railCollapsed: boolean
  onRailToggle: () => void
  onCreateGroup: () => void
  onCreateAgent: () => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <div
      data-slot="inbox-pane-header"
      className="flex h-14 shrink-0 items-center gap-2 border-b border-border/60 px-3"
    >
      <Button
        variant="ghost"
        size="icon"
        className="shrink-0 text-muted-foreground"
        aria-pressed={railCollapsed}
        aria-label={
          railCollapsed ? t("scopeRailExpand") : t("scopeRailCollapse")
        }
        title={railCollapsed ? t("scopeRailExpand") : t("scopeRailCollapse")}
        onClick={onRailToggle}
      >
        <PanelLeftIcon className="size-5" />
      </Button>
      <div className="relative min-w-0 flex-1">
        <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          disabled
          aria-label={t("searchLabel")}
          className="h-9 w-full rounded-md border border-transparent bg-muted px-8 text-sm text-foreground opacity-50"
        />
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0 bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground"
            aria-label={t("newConversation")}
            title={t("newConversation")}
          >
            <PlusIcon />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="min-w-52">
          <DropdownMenuItem onSelect={onCreateAgent}>
            {t("newAgentConversation")}
          </DropdownMenuItem>
          <DropdownMenuItem className="gap-2" onSelect={onCreateGroup}>
            <span className="min-w-0 flex-1 truncate">
              {t("newGroupConversation")}
            </span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
