/** 客户范围的处理归属视图切换。 */
import { CheckIcon, ChevronDownIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  CustomerInboxView,
  OrganizationIdentityType,
  type InboxAssignee,
} from "@/api"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

/** 客户范围的处理归属视图；同事视图在下拉中继续选择具体客服。 */
export function InboxCustomerQueueFilter({
  view,
  assigneeIdentityId,
  assignees,
  currentIdentityId,
  onChange,
}: {
  view: CustomerInboxView
  assigneeIdentityId: string
  assignees: InboxAssignee[]
  currentIdentityId: string
  onChange: (view: CustomerInboxView, assigneeIdentityId?: string) => void
}) {
  const { t } = useTranslation("inbox")
  const coworkers = assignees.filter(
    (assignee) => assignee.identityId !== currentIdentityId,
  )
  const selectedCoworker = coworkers.find(
    (assignee) => assignee.identityId === assigneeIdentityId,
  )
  const segments = [
    {
      id: CustomerInboxView.CustomerInboxViewQueue,
      label: t("queueFilterQueue"),
    },
    {
      id: CustomerInboxView.CustomerInboxViewMine,
      label: t("queueFilterMine"),
    },
  ] as const

  function tabClass(active: boolean) {
    return cn(
      "relative h-9 min-w-0 flex-1 rounded-md px-1 text-center text-sm font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
      active
        ? "text-foreground"
        : "text-muted-foreground hover:text-foreground",
    )
  }

  function activeIndicator(active: boolean) {
    return active ? (
      <span
        aria-hidden="true"
        className="absolute right-1 -bottom-px left-1 h-0.5 rounded bg-primary"
      />
    ) : null
  }

  return (
    <div
      role="tablist"
      aria-label={t("queueFilterLabel")}
      className="flex shrink-0 items-stretch border-b border-border/60 px-2 pt-2"
    >
      {segments.map((segment) => {
        const active = view === segment.id
        return (
          <button
            key={segment.id}
            type="button"
            role="tab"
            aria-selected={active}
            className={tabClass(active)}
            onClick={() => onChange(segment.id)}
          >
            <span className="block truncate">{segment.label}</span>
            {activeIndicator(active)}
          </button>
        )
      })}
      <button type="button" role="tab" aria-selected={false} disabled className={tabClass(false)}>
        <span className="block truncate">{t("queueFilterMentions")}</span>
      </button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            role="tab"
            aria-selected={
              view === CustomerInboxView.CustomerInboxViewCoworkers
            }
            title={selectedCoworker?.displayName ?? t("queueFilterColleague")}
            className={tabClass(
              view === CustomerInboxView.CustomerInboxViewCoworkers,
            )}
          >
            <span className="flex min-w-0 items-center justify-center gap-0.5">
              <span className="truncate">{t("queueFilterColleague")}</span>
              <ChevronDownIcon className="size-3.5 opacity-70" />
            </span>
            {activeIndicator(
              view === CustomerInboxView.CustomerInboxViewCoworkers,
            )}
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="center" className="min-w-48">
          <DropdownMenuItem
            className={cn(
              view === CustomerInboxView.CustomerInboxViewCoworkers &&
                !assigneeIdentityId &&
                "bg-muted text-foreground",
            )}
            onSelect={() =>
              onChange(CustomerInboxView.CustomerInboxViewCoworkers, "")
            }
          >
            <span className="min-w-0 flex-1 truncate">
              {t("queueFilterAllCoworkers")}
            </span>
            {view === CustomerInboxView.CustomerInboxViewCoworkers &&
            !assigneeIdentityId ? (
              <CheckIcon className="size-4" />
            ) : null}
          </DropdownMenuItem>
          {coworkers.map((assignee) => (
            <DropdownMenuItem
              key={assignee.identityId}
              className={cn(
                assigneeIdentityId === assignee.identityId &&
                  "bg-muted text-foreground",
              )}
              onSelect={() =>
                onChange(
                  CustomerInboxView.CustomerInboxViewCoworkers,
                  assignee.identityId,
                )
              }
            >
              <span className="min-w-0 flex-1 truncate">
                {assignee.displayName}
              </span>
              {assignee.type ===
              OrganizationIdentityType.OrganizationIdentityTypeAgent ? (
                <span className="text-xs text-muted-foreground">
                  {t("queueFilterAiEmployee")}
                </span>
              ) : null}
              {assigneeIdentityId === assignee.identityId ? (
                <CheckIcon className="size-4" />
              ) : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
