/** 会话列表列的顶部操作行。 */
import type { ReactNode } from "react"
import { PanelLeftIcon, PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { ListToolbarSearch } from "@/components/list-toolbar"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

/** 顶部操作行：展开范围栏、搜索占位、当前范围筛选和发起会话菜单。 */
export function InboxPaneTop({
  railCollapsed,
  onRailExpand,
  onCreateGroup,
  onCreateAgent,
  filter,
}: {
  railCollapsed: boolean
  onRailExpand: () => void
  onCreateGroup: () => void
  onCreateAgent: () => void
  filter: ReactNode
}) {
  const { t } = useTranslation("inbox")

  return (
    <div
      data-slot="inbox-pane-header"
      className="flex h-14 shrink-0 items-center gap-2 border-b border-border/60 px-3"
    >
      {railCollapsed ? (
        <Button
          data-slot="rail-toggle"
          variant="ghost"
          size="icon"
          className="shrink-0 text-muted-foreground"
          aria-label={t("scopeRailExpand")}
          title={t("scopeRailExpand")}
          onClick={onRailExpand}
        >
          <PanelLeftIcon className="size-5" />
        </Button>
      ) : null}
      <ListToolbarSearch
        type="text"
        disabled
        aria-label={t("searchLabel")}
        className="min-w-0 flex-1 sm:w-auto"
      />
      {filter}
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
