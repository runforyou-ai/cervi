/** 消息页中栏左缘的会话范围纵栏。 */
import { HeadsetIcon, MessagesSquareIcon, PanelLeftIcon, UsersRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { InboxScope } from "@/api"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const scopes = [
  {
    id: InboxScope.InboxScopeAll,
    labelKey: "scopeAll",
    icon: MessagesSquareIcon,
  },
  {
    id: InboxScope.InboxScopeCustomer,
    labelKey: "scopeCustomer",
    icon: HeadsetIcon,
  },
  {
    id: InboxScope.InboxScopeInternal,
    labelKey: "scopeInternal",
    icon: UsersRoundIcon,
  },
] as const

/** 中栏左缘的范围纵栏。 */
export function InboxScopeRail({
  scope,
  attentionUnreadCount,
  onScopeChange,
  onCollapse,
}: {
  scope: InboxScope
  attentionUnreadCount: number
  onScopeChange: (scope: InboxScope) => void
  onCollapse: () => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <div className="flex w-20 shrink-0 flex-col border-r border-border/70 bg-muted/30">
      <div className="flex h-14 shrink-0 items-center justify-center border-b border-border/60">
        <Button
          data-slot="rail-toggle"
          variant="ghost"
          size="icon"
          className="text-muted-foreground"
          aria-label={t("scopeRailCollapse")}
          title={t("scopeRailCollapse")}
          onClick={onCollapse}
        >
          <PanelLeftIcon className="size-5" />
        </Button>
      </div>
      <nav
        aria-label={t("scopeRailLabel")}
        className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-1.5 py-1.5"
      >
        {scopes.map((item) => (
          <button
            key={item.id}
            type="button"
            aria-pressed={scope === item.id}
            className={cn(
              "flex flex-col items-center gap-0.5 rounded-lg px-0 pt-1.5 pb-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
              scope === item.id &&
                "bg-accent font-medium text-accent-foreground hover:bg-accent hover:text-accent-foreground",
            )}
            onClick={() => onScopeChange(item.id)}
          >
            <span className="relative">
              <item.icon className="size-5" />
              {item.id === InboxScope.InboxScopeInternal &&
              attentionUnreadCount > 0 ? (
                <span
                  className="absolute -top-0.5 -right-1 size-2 rounded-full bg-destructive"
                  role="status"
                  aria-label={t("internalAttentionUnread", {
                    count: attentionUnreadCount,
                  })}
                />
              ) : null}
            </span>
            <span className="w-full truncate px-px text-center text-xs leading-tight">
              {t(item.labelKey)}
            </span>
          </button>
        ))}
      </nav>
    </div>
  )
}
