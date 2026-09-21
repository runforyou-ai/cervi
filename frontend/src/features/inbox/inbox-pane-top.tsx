/** 会话列表列的顶部操作行。 */
import type { ReactNode } from "react"
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { InboxScope } from "@/api"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

const scopes = [
  { id: InboxScope.InboxScopeAll, labelKey: "scopeAll" },
  { id: InboxScope.InboxScopeCustomer, labelKey: "scopeCustomer" },
  { id: InboxScope.InboxScopeInternal, labelKey: "scopeInternal" },
] as const

/** 顶部操作行：会话范围切换、当前范围筛选和发起会话菜单。 */
export function InboxPaneTop({
  scope,
  attentionUnreadCount,
  customerMentionedUnreadCount,
  onScopeChange,
  onCreateGroup,
  onCreateAgent,
  filter,
}: {
  scope: InboxScope
  attentionUnreadCount: number
  customerMentionedUnreadCount: number
  onScopeChange: (scope: InboxScope) => void
  onCreateGroup: () => void
  onCreateAgent: () => void
  filter: ReactNode
}) {
  const { t } = useTranslation("inbox")

  return (
    <div
      data-slot="inbox-pane-header"
      className="flex h-12 shrink-0 items-center gap-1.5 px-2.5"
    >
      <nav
        aria-label={t("scopeLabel")}
        className="flex min-w-0 flex-1 items-center gap-0.5"
      >
        {scopes.map((item) => {
          const unread =
            item.id === InboxScope.InboxScopeInternal
              ? attentionUnreadCount
              : item.id === InboxScope.InboxScopeCustomer
                ? customerMentionedUnreadCount
                : 0
          return (
            <button
              key={item.id}
              type="button"
              aria-pressed={scope === item.id}
              className={cn(
                "flex h-7 min-w-0 items-center gap-1 rounded-md px-2 text-[13px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                scope === item.id &&
                  "bg-accent font-medium text-accent-foreground hover:bg-accent hover:text-accent-foreground",
              )}
              onClick={() => onScopeChange(item.id)}
            >
              <span className="truncate">{t(item.labelKey)}</span>
              {unread > 0 ? (
                <span
                  className="size-1.5 shrink-0 rounded-full bg-destructive"
                  role="status"
                  aria-label={t(
                    item.id === InboxScope.InboxScopeInternal
                      ? "internalAttentionUnread"
                      : "customerMentionUnread",
                    { count: unread },
                  )}
                />
              ) : null}
            </button>
          )
        })}
      </nav>
      {/* 漏斗与加号字形两侧留白偏多，间距收为零抵消，视觉上与会话头的图标按钮一致。 */}
      <div className="flex shrink-0 items-center">
        {filter}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              className="shrink-0 text-muted-foreground"
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
    </div>
  )
}
