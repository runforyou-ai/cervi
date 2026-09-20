/** 会话列表列的顶部操作行。 */
import type { ReactNode } from "react"
import { PanelLeftIcon, PlusIcon, SearchIcon, XIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import type { InboxSearchState } from "@/features/inbox/use-inbox-search"
import { cn } from "@/lib/utils"

/** 顶部操作行：展开范围栏、搜索框、当前范围筛选和发起会话菜单。 */
export function InboxPaneTop({
  railCollapsed,
  onRailExpand,
  onCreateGroup,
  onCreateAgent,
  filter,
  search,
}: {
  railCollapsed: boolean
  onRailExpand: () => void
  onCreateGroup: () => void
  onCreateAgent: () => void
  filter: ReactNode
  search: InboxSearchState
}) {
  const { t } = useTranslation("inbox")

  return (
    <div
      data-slot="inbox-pane-header"
      className="flex h-12 shrink-0 items-center gap-1.5 px-2.5"
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
          <PanelLeftIcon className="size-[18px]" />
        </Button>
      ) : null}
      <div className="relative min-w-0 flex-1">
        <SearchIcon className="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          ref={search.inputRef}
          type="text"
          value={search.text}
          aria-label={t("searchLabel")}
          title={t("searchShortcut")}
          className={cn("h-7 pl-7.5", search.active && "pr-7")}
          onFocus={search.handleFocus}
          onChange={(event) => search.setText(event.target.value)}
          onKeyDown={search.handleKeyDown}
        />
        {search.active ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            className="absolute top-1/2 right-1.5 -translate-y-1/2 text-muted-foreground"
            aria-label={t("searchExit")}
            title={t("searchExit")}
            onClick={search.exit}
          >
            <XIcon />
          </Button>
        ) : null}
      </div>
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
