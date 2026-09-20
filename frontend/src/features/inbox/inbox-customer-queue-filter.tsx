/** 客户范围的处理归属视图切换。 */
import type { ReactNode } from "react"
import { CheckIcon, ChevronDownIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  CustomerInboxView,
  CustomerQueueFilter,
  OrganizationIdentityType,
  type InboxAssignee,
  type ServiceQueueTeam,
} from "@/api"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

/** 客户视图切换的筛选变更。 */
export type CustomerQueueFilterChange = {
  customerView: CustomerInboxView
  assigneeIdentityId?: string
  queueFilter?: CustomerQueueFilter
  queueTeamId?: string
}

/** 页签按钮样式，选中时以底部指示条标记。 */
function tabClass(active: boolean) {
  return cn(
    "relative h-9 min-w-0 flex-1 rounded-md px-1 text-center text-sm font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
    active ? "text-foreground" : "text-muted-foreground hover:text-foreground",
  )
}

/** 选中页签底部的指示条。 */
function ActiveIndicator({ active }: { active: boolean }) {
  return active ? (
    <span
      aria-hidden="true"
      className="absolute right-1 -bottom-px left-1 h-0.5 rounded bg-primary"
    />
  ) : null
}

/** 带下拉的页签触发按钮。 */
function DropdownTab({
  active,
  label,
  title,
  children,
}: {
  active: boolean
  label: string
  title: string
  children: ReactNode
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          role="tab"
          aria-selected={active}
          title={title}
          className={tabClass(active)}
        >
          <span className="flex min-w-0 items-center justify-center gap-0.5">
            <span className="truncate">{label}</span>
            <ChevronDownIcon className="size-3.5 opacity-70" />
          </span>
          <ActiveIndicator active={active} />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="center" className="min-w-48">
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** 下拉中的一项，选中项右侧显示对勾。 */
function DropdownTabItem({
  selected,
  label,
  hint,
  onSelect,
}: {
  selected: boolean
  label: string
  hint?: string
  onSelect: () => void
}) {
  return (
    <DropdownMenuItem
      className={cn(selected && "bg-muted text-foreground")}
      onSelect={onSelect}
    >
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {hint ? <span className="text-xs text-muted-foreground">{hint}</span> : null}
      {selected ? <CheckIcon className="size-4" /> : null}
    </DropdownMenuItem>
  )
}

/** 待分配视图，下拉中按全部队列、公共队列和各团队队列筛选。 */
function QueueTab({
  active,
  queueFilter,
  queueTeamId,
  teams,
  onChange,
}: {
  active: boolean
  queueFilter: CustomerQueueFilter
  queueTeamId: string
  teams: ServiceQueueTeam[]
  onChange: (change: CustomerQueueFilterChange) => void
}) {
  const { t } = useTranslation("inbox")
  const selectedTeam = teams.find((team) => team.id === queueTeamId)
  const label =
    active && queueFilter === CustomerQueueFilter.CustomerQueueFilterPublic
      ? t("queueFilterPublicQueue")
      : active && selectedTeam
        ? selectedTeam.name
        : t("queueFilterQueue")
  const select = (filter: CustomerQueueFilter, teamId = "") =>
    onChange({
      customerView: CustomerInboxView.CustomerInboxViewQueue,
      queueFilter: filter,
      queueTeamId: teamId,
    })
  return (
    <DropdownTab active={active} label={label} title={t("queueFilterQueue")}>
      <DropdownTabItem
        selected={
          active && queueFilter === CustomerQueueFilter.CustomerQueueFilterAll
        }
        label={t("queueFilterAllQueues")}
        onSelect={() => select(CustomerQueueFilter.CustomerQueueFilterAll)}
      />
      <DropdownTabItem
        selected={
          active && queueFilter === CustomerQueueFilter.CustomerQueueFilterPublic
        }
        label={t("queueFilterPublicQueue")}
        onSelect={() => select(CustomerQueueFilter.CustomerQueueFilterPublic)}
      />
      {teams.map((team) => (
        <DropdownTabItem
          key={team.id}
          selected={active && queueTeamId === team.id}
          label={team.name}
          onSelect={() =>
            select(CustomerQueueFilter.CustomerQueueFilterTeam, team.id)
          }
        />
      ))}
    </DropdownTab>
  )
}

/** 同事视图，下拉中继续按具体客服筛选。 */
function CoworkerTab({
  active,
  assigneeIdentityId,
  coworkers,
  onChange,
}: {
  active: boolean
  assigneeIdentityId: string
  coworkers: InboxAssignee[]
  onChange: (change: CustomerQueueFilterChange) => void
}) {
  const { t } = useTranslation("inbox")
  const selected = coworkers.find(
    (assignee) => assignee.identityId === assigneeIdentityId,
  )
  const select = (identityId: string) =>
    onChange({
      customerView: CustomerInboxView.CustomerInboxViewCoworkers,
      assigneeIdentityId: identityId,
    })
  return (
    <DropdownTab
      active={active}
      label={t("queueFilterColleague")}
      title={selected?.displayName ?? t("queueFilterColleague")}
    >
      <DropdownTabItem
        selected={active && !assigneeIdentityId}
        label={t("queueFilterAllCoworkers")}
        onSelect={() => select("")}
      />
      {coworkers.map((assignee) => (
        <DropdownTabItem
          key={assignee.identityId}
          selected={assigneeIdentityId === assignee.identityId}
          label={assignee.displayName}
          hint={
            assignee.type ===
            OrganizationIdentityType.OrganizationIdentityTypeAgent
              ? t("queueFilterAiEmployee")
              : undefined
          }
          onSelect={() => select(assignee.identityId)}
        />
      ))}
    </DropdownTab>
  )
}

/** 客户范围的处理归属视图；待分配与同事视图在下拉中继续选择队列和客服。 */
export function InboxCustomerQueueFilter({
  view,
  queueFilter,
  queueTeamId,
  queueTeams,
  assigneeIdentityId,
  assignees,
  currentIdentityId,
  onChange,
}: {
  view: CustomerInboxView
  queueFilter: CustomerQueueFilter
  queueTeamId: string
  queueTeams: ServiceQueueTeam[]
  assigneeIdentityId: string
  assignees: InboxAssignee[]
  currentIdentityId: string
  onChange: (change: CustomerQueueFilterChange) => void
}) {
  const { t } = useTranslation("inbox")
  const coworkers = assignees.filter(
    (assignee) => assignee.identityId !== currentIdentityId,
  )
  const segments = [
    { id: CustomerInboxView.CustomerInboxViewMine, label: t("queueFilterMine") },
    {
      id: CustomerInboxView.CustomerInboxViewMentioned,
      label: t("queueFilterMentions"),
    },
  ] as const

  return (
    <div
      role="tablist"
      aria-label={t("queueFilterLabel")}
      className="flex shrink-0 items-stretch border-b border-border/60 px-2 pt-2"
    >
      <QueueTab
        active={view === CustomerInboxView.CustomerInboxViewQueue}
        queueFilter={queueFilter}
        queueTeamId={queueTeamId}
        teams={queueTeams}
        onChange={onChange}
      />
      {segments.map((segment) => {
        const active = view === segment.id
        return (
          <button
            key={segment.id}
            type="button"
            role="tab"
            aria-selected={active}
            className={tabClass(active)}
            onClick={() => onChange({ customerView: segment.id })}
          >
            <span className="block truncate">{segment.label}</span>
            <ActiveIndicator active={active} />
          </button>
        )
      })}
      <CoworkerTab
        active={view === CustomerInboxView.CustomerInboxViewCoworkers}
        assigneeIdentityId={assigneeIdentityId}
        coworkers={coworkers}
        onChange={onChange}
      />
    </div>
  )
}
