/** 通讯录列表、筛选、详情和回收站。 */
import {
  forwardRef,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ButtonHTMLAttributes,
} from "react"
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  ContactRoundIcon,
  GlobeIcon,
  LoaderCircleIcon,
  MoreHorizontalIcon,
  PanelsTopLeftIcon,
  PlusIcon,
  UsersIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams, useSearchParams } from "react-router"
import { toast } from "sonner"

import {
  ChannelType,
  ContactMethodType,
  ContactSort,
  ContactStage,
  OrganizationIdentityType,
  UserStatus,
  WorkStatus,
  deactivateAgent,
  deactivateUser,
  deleteContact,
  getContact,
  getAgent,
  getUser,
  listChannels,
  listContacts,
  listDeletedContacts,
  listAgents,
  listRoles,
  listTeamMembers,
  listTeams,
  listUsers,
  reactivateUser,
  reactivateAgent,
  deleteTeam,
  removeTeamMembers,
  restoreContact,
  type ChannelSummary,
  type ContactDetail,
  type ContactListResponse,
  type ContactSummary,
  type AgentData,
  type UserData,
  type PageInfo,
  type RoleData,
  type Team,
  type TeamMember,
  type TeamSummary,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { optionalWailsEnum } from "@/lib/wails-enum"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { PagePaneNav, PageSplit } from "@/components/page-split"
import { SelectableText } from "@/components/selectable-text"
import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import { NativeSelect } from "@/components/ui/native-select"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { ContactForm } from "@/features/contacts/contact-form"
import { ContactDetailView } from "@/features/contacts/contact-detail"
import { AgentForm } from "@/features/contacts/agent-form"
import { AgentDetailView } from "@/features/contacts/agent-detail"
import { MemberDetailView } from "@/features/contacts/member-detail"
import { MemberForm } from "@/features/contacts/member-form"
import { TeamForm } from "@/features/contacts/team-form"
import { TeamMemberPicker } from "@/features/contacts/team-member-picker"
import { roleDisplayName } from "@/features/roles/role-labels"
import { useWorkspace } from "@/features/workspace/workspace-context"
import {
  channelTypeLabel,
  userStatusLabel,
} from "@/features/contacts/contact-labels"
import { WorkStatusBadge, workStatusLabel } from "@/features/users/work-status"
import { useDateTime } from "@/hooks/use-date-time"
import { cn } from "@/lib/utils"

type ContactScope = "employees" | "agents" | "team" | "external"

type LoadState = "loading" | "ready" | "error"

const contactNavHoverClass =
  "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
const contactNavLeafActiveClass =
  "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
const contactNavPathActiveClass = "font-medium text-sidebar-accent-foreground"
const contactNavSubitemClass =
  "flex h-8 w-full items-center gap-2 rounded-md py-1.5 pr-2 text-left text-sm text-muted-foreground transition-colors"

/** 显示团队摘要，并在悬停或聚焦时逐行列出全部团队。 */
function JoinedTeamsCell({ teams }: { teams: TeamSummary[] }) {
  const { t } = useTranslation("contacts")

  if (teams.length === 0) return "—"

  const summary =
    teams.length === 1
      ? teams[0].name
      : teams.length === 2
        ? t("teams.joinedPair", {
            first: teams[0].name,
            second: teams[1].name,
          })
        : t("teams.joinedSummary", {
            first: teams[0].name,
            second: teams[1].name,
            count: teams.length,
          })

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          tabIndex={0}
          className="block max-w-xs cursor-help truncate outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {summary}
        </span>
      </TooltipTrigger>
      <TooltipContent side="bottom" sideOffset={4} className="max-w-xs">
        <ul className="grid gap-1 text-left">
          {teams.map((team) => (
            <li key={team.id} className="break-words">
              {team.name}
            </li>
          ))}
        </ul>
      </TooltipContent>
    </Tooltip>
  )
}

/** 通讯录子分类按钮。 */
const SubscopeButton = forwardRef<
  HTMLButtonElement,
  {
    active: boolean
    nested?: boolean
    icon?: typeof GlobeIcon
  } & ButtonHTMLAttributes<HTMLButtonElement>
>(function SubscopeButton(
  { active, children, nested = false, icon: Icon, className, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type="button"
      className={cn(
        contactNavSubitemClass,
        contactNavHoverClass,
        nested ? "pl-14" : "pl-8",
        active && contactNavLeafActiveClass,
        className,
      )}
      {...props}
    >
      {Icon ? <Icon className="size-3.5 shrink-0" /> : null}
      <span className="truncate">{children}</span>
    </button>
  )
})

/** 通讯录分类和来源渠道筛选。 */
function ContactScopeSidebar({
  scope,
  deleted,
  channelId,
  channels,
  teamId,
  teams,
}: {
  scope: ContactScope
  deleted: boolean
  channelId: string
  channels: ChannelSummary[]
  teamId: string
  teams: Team[]
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const groupedChannels = useMemo(() => {
    const groups = new Map<ChannelType, ChannelSummary[]>()
    for (const channel of channels) {
      groups.set(channel.type, [...(groups.get(channel.type) ?? []), channel])
    }
    return [...groups.entries()]
  }, [channels])

  return (
    <PagePaneNav
      label={t("scopeNavigation")}
      title={t("title")}
      action={
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              className="bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground"
              aria-label={t("add.label")}
              title={t("add.label")}
            >
              <PlusIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="start">
            <DropdownMenuItem
              onSelect={() =>
                navigate(
                  scope === "team" && teamId
                    ? `/contacts/teams/${encodeURIComponent(teamId)}?new=1`
                    : "/contacts/employees?new=1",
                )
              }
            >
              {t("add.member")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={() =>
                navigate(
                  scope === "team" && teamId
                    ? `/contacts/teams/${encodeURIComponent(teamId)}?newAgent=1`
                    : "/contacts/ai-employees?newAgent=1",
                )
              }
            >
              {t("add.agent")}
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={channels.length === 0}
              onSelect={() => navigate("/contacts/external?new=1")}
            >
              {t("add.external")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={() => navigate("/contacts/employees?newTeam=1")}
            >
              {t("teams.create")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      }
    >
      <Collapsible defaultOpen>
        <CollapsibleTrigger asChild>
          <button
            type="button"
            className={cn(
              "group flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors",
              contactNavHoverClass,
              scope !== "external" && contactNavPathActiveClass,
            )}
          >
            <UsersIcon className="size-4" />
            <span>{t("scopes.members")}</span>
            <ChevronRightIcon className="ml-auto size-4 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent className="flex flex-col gap-0.5">
          <SubscopeButton
            active={scope === "employees"}
            onClick={() => navigate("/contacts/employees")}
          >
            {t("scopes.employees")}
          </SubscopeButton>
          <SubscopeButton
            active={scope === "agents"}
            onClick={() => navigate("/contacts/ai-employees")}
          >
            {t("scopes.agents")}
          </SubscopeButton>
          <Collapsible defaultOpen>
            <CollapsibleTrigger asChild>
              <button
                type="button"
                className={cn(
                  "group pl-8",
                  contactNavSubitemClass,
                  contactNavHoverClass,
                  scope === "team" && contactNavPathActiveClass,
                )}
              >
                <span>{t("scopes.teams")}</span>
                <ChevronRightIcon className="ml-auto size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
              </button>
            </CollapsibleTrigger>
            <CollapsibleContent className="flex flex-col gap-0.5">
              {teams.map((team) => (
                <SubscopeButton
                  key={team.id}
                  active={scope === "team" && teamId === team.id}
                  icon={PanelsTopLeftIcon}
                  nested
                  onClick={() => navigate(`/contacts/teams/${team.id}`)}
                >
                  {team.name}
                </SubscopeButton>
              ))}
            </CollapsibleContent>
          </Collapsible>
        </CollapsibleContent>
      </Collapsible>

      <Collapsible defaultOpen>
        <CollapsibleTrigger asChild>
          <button
            type="button"
            className={cn(
              "group flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors",
              contactNavHoverClass,
              scope === "external" && contactNavPathActiveClass,
            )}
          >
            <ContactRoundIcon className="size-4" />
            <span>{t("scopes.external")}</span>
            <ChevronRightIcon className="ml-auto size-4 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent className="flex flex-col gap-0.5">
          <SubscopeButton
            active={scope === "external" && !deleted && !channelId}
            onClick={() => navigate("/contacts/external")}
          >
            {t("all")}
          </SubscopeButton>
          {groupedChannels.map(([type, items]) => (
            <Collapsible key={type} defaultOpen>
              <CollapsibleTrigger asChild>
                <button
                  type="button"
                  className={cn(
                    "group pl-8",
                    contactNavSubitemClass,
                    contactNavHoverClass,
                    scope === "external" &&
                      items.some((channel) => channel.id === channelId) &&
                      contactNavPathActiveClass,
                  )}
                >
                  <span>{channelTypeLabel(type, t)}</span>
                  <ChevronRightIcon className="ml-auto size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
                </button>
              </CollapsibleTrigger>
              <CollapsibleContent className="flex flex-col gap-0.5">
                {items.map((channel) => (
                  <SubscopeButton
                    key={channel.id}
                    active={scope === "external" && channelId === channel.id}
                    icon={
                      type === ChannelType.ChannelTypeWebsite
                        ? GlobeIcon
                        : undefined
                    }
                    nested
                    onClick={() =>
                      navigate(`/contacts/external?channelId=${channel.id}`)
                    }
                  >
                    {channel.name}
                  </SubscopeButton>
                ))}
              </CollapsibleContent>
            </Collapsible>
          ))}
        </CollapsibleContent>
      </Collapsible>
    </PagePaneNav>
  )
}

/** 联系人阶段标签。 */
function StageLabel({ stage }: { stage: ContactStage }) {
  const { t } = useTranslation("contacts")
  if (!stage) return null
  return (
    <SelectableText className="inline-flex rounded-md bg-secondary px-2 py-0.5 text-xs font-medium text-secondary-foreground">
      {t(`stages.${stage}`)}
    </SelectableText>
  )
}

/** 显示账号状态徽标。 */
function UserStatusBadge({
  status,
  label,
}: {
  status: UserStatus
  label: string
}) {
  const active = status === UserStatus.UserStatusActive

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
        active
          ? "bg-success/15 text-success"
          : "bg-muted text-muted-foreground",
      )}
    >
      {label}
    </span>
  )
}

/** 联系人列表分页。 */
function PageControls({
  page,
  onPageChange,
}: {
  page: PageInfo
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation("contacts")
  const totalPages = Math.max(1, Math.ceil(page.total / page.size))
  return (
    <div className="flex items-center justify-between border-t px-4 py-3 text-sm text-muted-foreground">
      <span>{t("pagination.total", { count: page.total })}</span>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={page.number <= 1}
          onClick={() => onPageChange(page.number - 1)}
        >
          <ChevronLeftIcon />
          {t("pagination.previous")}
        </Button>
        <span>
          {t("pagination.page", { current: page.number, total: totalPages })}
        </span>
        <Button
          variant="outline"
          size="sm"
          disabled={page.number >= totalPages}
          onClick={() => onPageChange(page.number + 1)}
        >
          {t("pagination.next")}
          <ChevronRightIcon />
        </Button>
      </div>
    </div>
  )
}

/** 按分类列出通讯录。 */
export function ContactsPage({ scope }: { scope: ContactScope }) {
  const { t } = useTranslation("contacts")
  const { t: tCommon } = useTranslation("common")
  const { identity, updateUser: updateWorkspaceUser } = useWorkspace()
  const navigate = useNavigate()
  const { teamId = "" } = useParams()
  const { formatDateTime } = useDateTime()
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get("q") ?? ""
  const [search, setSearch] = useState(query)
  const [channels, setChannels] = useState<ChannelSummary[]>([])
  const [contacts, setContacts] = useState<ContactSummary[]>([])
  const [users, setUsers] = useState<UserData[]>([])
  const [agents, setAgents] = useState<AgentData[]>([])
  const [teamMembers, setTeamMembers] = useState<TeamMember[]>([])
  const [roles, setRoles] = useState<RoleData[]>([])
  const [teams, setTeams] = useState<Team[]>([])
  const [page, setPage] = useState<PageInfo>({ number: 1, size: 50, total: 0 })
  const [loadState, setLoadState] = useState<LoadState>("loading")
  const [refreshVersion, setRefreshVersion] = useState(0)
  const [detail, setDetail] = useState<ContactDetail | null>(null)
  const [detailUser, setDetailUser] = useState<UserData | null>(null)
  const [detailAgent, setDetailAgent] = useState<AgentData | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [deletingContact, setDeletingContact] = useState<ContactSummary | null>(
    null,
  )
  const [restoringContact, setRestoringContact] =
    useState<ContactSummary | null>(null)
  const [changingUserStatus, setChangingUserStatus] =
    useState<UserData | null>(null)
  const [changingAgentStatus, setChangingAgentStatus] =
    useState<AgentData | null>(null)
  const [deletingTeam, setDeletingTeam] = useState<Team | null>(null)
  const [removingTeamMembers, setRemovingTeamMembers] = useState<
    TeamMember[]
  >([])
  const [selectedTeamMemberIdentityIDs, setSelectedTeamMemberIdentityIDs] =
    useState<Set<string>>(new Set())
  const [deleting, setDeleting] = useState(false)
  const detailTitleRef = useRef<HTMLHeadingElement>(null)
  const catalogRequestID = useRef(0)
  const listRequestID = useRef(0)
  const detailRequestID = useRef(0)
  const selected = searchParams.get("selected") ?? ""
  const deleted = scope === "external" && searchParams.get("view") === "trash"
  const creating = searchParams.get("new") === "1"
  const creatingAgent = searchParams.get("newAgent") === "1"
  const creatingTeam = searchParams.get("newTeam") === "1"
  const editingTeam = searchParams.get("editTeam") === "1"
  const addingTeamMembers = searchParams.get("addMembers") === "1"
  const channelId = searchParams.get("channelId") ?? ""
  const stage = optionalWailsEnum(ContactStage, searchParams.get("stage"))
  const methodType = optionalWailsEnum(
    ContactMethodType,
    searchParams.get("methodType"),
  )
  const sort =
    optionalWailsEnum(ContactSort, searchParams.get("sort")) ??
    ContactSort.ContactSortCreatedAtDescending
  const status =
    optionalWailsEnum(UserStatus, searchParams.get("status")) ??
    UserStatus.UserStatusActive
  const workStatus = optionalWailsEnum(
    WorkStatus,
    searchParams.get("workStatus"),
  )
  const roleId = searchParams.get("roleId") ?? ""
  const selectedTeam = teams.find((team) => team.id === teamId)
  const currentPage = Number(searchParams.get("page") ?? "1") || 1

  /** 当前用户使用工作台中的即时状态，其他成员使用目录查询结果。 */
  function memberWorkStatus(user: UserData) {
    return user.id === identity.user.id
      ? identity.user.workStatus
      : user.workStatus
  }

  /** 当前用户使用工作台中的即时状态，其他团队成员使用列表结果。 */
  function identityWorkStatus(member: TeamMember) {
    return member.identityType ===
      OrganizationIdentityType.OrganizationIdentityTypeUser &&
      member.identityId === identity.user.identityId
      ? identity.user.workStatus
      : member.workStatus
  }

  /** 更新列表查询参数。 */
  const setParameters = useCallback(
    (changes: Record<string, string | null>) => {
      setSearchParams((current) => {
        const next = new URLSearchParams(current)
        for (const [name, value] of Object.entries(changes)) {
          if (!value) {
            next.delete(name)
          } else {
            next.set(name, value)
          }
        }
        return next
      })
    },
    [setSearchParams],
  )

  useEffect(() => setSearch(query), [query])
  useEffect(() => {
    setSelectedTeamMemberIdentityIDs(new Set())
  }, [currentPage, query, roleId, status, teamId, workStatus])
  useEffect(() => {
    const timeout = window.setTimeout(() => {
      if (search !== query) {
        setParameters({ q: search || null, page: null, selected: null })
      }
    }, 300)
    return () => window.clearTimeout(timeout)
  }, [query, search, setParameters])

  useEffect(() => {
    const requestID = ++catalogRequestID.current
    void Promise.all([
      listChannels(),
      listRoles(),
      listTeams({ pageSize: 100 }),
    ])
      .then(([channelItems, roleOutput, teamOutput]) => {
        if (requestID !== catalogRequestID.current) return
        setChannels(channelItems)
        setRoles(roleOutput.roles)
        setTeams(teamOutput.teams)
      })
      .catch((error: unknown) => {
        if (requestID !== catalogRequestID.current) return
        console.warn("通讯录筛选数据加载失败", error)
        setChannels([])
        setRoles([])
        setTeams([])
      })
    return () => {
      catalogRequestID.current += 1
    }
  }, [])

  /** 按当前范围加载联系人或企业成员列表。 */
  const loadList = useCallback(async () => {
    const requestID = ++listRequestID.current
    setLoadState("loading")
    try {
      if (scope === "external") {
        const loader = deleted ? listDeletedContacts : listContacts
        const response: ContactListResponse = await loader({
          query,
          stage,
          channelId: deleted ? "" : channelId,
          methodType,
          sort,
          page: currentPage,
          pageSize: 50,
        })
        if (requestID !== listRequestID.current) return
        setContacts(response.contacts)
        setPage(response.page)
      } else if (scope === "employees") {
        const response = await listUsers({
          query,
          status,
          roleId,
          page: currentPage,
          pageSize: 50,
        })
        if (requestID !== listRequestID.current) return
        setUsers(response.users)
        setPage(response.page)
      } else if (scope === "agents") {
        const response = await listAgents({
          query,
          status,
          page: currentPage,
          pageSize: 50,
        })
        if (requestID !== listRequestID.current) return
        setAgents(response.agents)
        setPage(response.page)
      } else {
        const response = await listTeamMembers(teamId, {
          query,
          workStatus,
          page: currentPage,
          pageSize: 50,
        })
        if (requestID !== listRequestID.current) return
        setTeamMembers(response.members)
        setPage(response.page)
      }
      setLoadState("ready")
    } catch (error) {
      if (requestID !== listRequestID.current) return
      if (recoverSession(error, navigate)) return
      console.warn("联系人列表加载失败", error)
      setLoadState("error")
    }
  }, [
    channelId,
    currentPage,
    deleted,
    methodType,
    navigate,
    query,
    roleId,
    scope,
    sort,
    stage,
    status,
    teamId,
    workStatus,
  ])

  useEffect(() => {
    void loadList()
    return () => {
      listRequestID.current += 1
    }
  }, [loadList, refreshVersion])

  useEffect(() => {
    setDetail(null)
    setDetailUser(null)
    setDetailAgent(null)
    if (!selected || scope === "team") {
      return
    }
    const requestID = ++detailRequestID.current
    setDetailLoading(true)
    const loader = scope === "external"
      ? getContact(selected).then((output) => {
          if (requestID === detailRequestID.current) setDetail(output)
        })
      : scope === "employees"
        ? getUser(selected).then((output) => {
            if (requestID === detailRequestID.current) setDetailUser(output)
          })
        : getAgent(selected).then((output) => {
            if (requestID === detailRequestID.current) setDetailAgent(output)
          })
    void loader
      .catch((error: unknown) => {
        if (requestID !== detailRequestID.current) return
        if (recoverSession(error, navigate)) {
          return
        }
        console.warn("联系人详情加载失败", error)
        toast.error(t("detail.loadError"))
        setParameters({ selected: null })
      })
      .finally(() => {
        if (requestID === detailRequestID.current) setDetailLoading(false)
      })
    return () => {
      detailRequestID.current += 1
    }
  }, [navigate, scope, selected, setParameters, t])

  const selectedChannel = channels.find((channel) => channel.id === channelId)
  const title =
    scope === "employees"
      ? t("scopes.employees")
      : scope === "agents"
        ? t("scopes.agents")
        : scope === "team"
          ? (selectedTeam?.name ?? t("scopes.teams"))
          : (selectedChannel?.name ?? t("scopes.external"))
  const mobileScope =
    scope === "employees"
      ? "employees"
      : scope === "agents"
        ? "agents"
        : scope === "team"
          ? `team:${teamId}`
          : channelId
            ? `channel:${channelId}`
            : "external"

  /** 窄视口下切换联系人范围。 */
  function changeMobileScope(value: string) {
    if (value === "employees") {
      navigate("/contacts/employees")
    } else if (value === "agents") {
      navigate("/contacts/ai-employees")
    } else if (value.startsWith("team:")) {
      navigate(`/contacts/teams/${value.slice("team:".length)}`)
    } else if (value.startsWith("channel:")) {
      navigate(`/contacts/external?channelId=${value.slice("channel:".length)}`)
    } else {
      navigate("/contacts/external")
    }
  }

  /** 关闭联系人详情。 */
  function closeDetail() {
    setParameters({ selected: null, new: null })
    setDetail(null)
    setDetailUser(null)
    setDetailAgent(null)
  }

  /** 刷新列表并关闭详情。 */
  function refreshAndClose() {
    closeDetail()
    setRefreshVersion((current) => current + 1)
  }

  /** 将联系人移入回收站。 */
  async function removeContact() {
    if (!deletingContact) {
      return
    }
    setDeleting(true)
    try {
      await deleteContact(deletingContact.id)
      console.info("联系人已移入回收站", { contact_id: deletingContact.id })
      toast.success(t("delete.success"))
      setDeletingContact(null)
      if (selected === deletingContact.id) {
        closeDetail()
      }
      setRefreshVersion((current) => current + 1)
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("删除联系人失败", error)
      toast.error(t("delete.error"))
    } finally {
      setDeleting(false)
    }
  }

  /** 恢复联系人。 */
  async function restore() {
    if (!restoringContact) return
    setDeleting(true)
    try {
      await restoreContact(restoringContact.id)
      console.info("联系人已恢复", { contact_id: restoringContact.id })
      toast.success(t("trash.restored"))
      setRestoringContact(null)
      setRefreshVersion((current) => current + 1)
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("恢复联系人失败", error)
      toast.error(t("trash.restoreError"))
    } finally {
      setDeleting(false)
    }
  }

  /** 禁用用户账号或恢复为正常状态。 */
  async function changeUserStatus() {
    if (!changingUserStatus) return
    setDeleting(true)
    try {
      const saved =
        changingUserStatus.status === UserStatus.UserStatusActive
          ? await deactivateUser(changingUserStatus.id)
          : await reactivateUser(changingUserStatus.id)
      console.info("企业成员账号状态已修改", {
        identity_id: saved.identityId,
        user_id: saved.id,
        status: saved.status,
      })
      toast.success(
        t(
          changingUserStatus.status === UserStatus.UserStatusActive
            ? "members.status.deactivated"
            : "members.status.reactivated",
        ),
      )
      setChangingUserStatus(null)
      if (detailUser?.id === saved.id) {
        setDetailUser(saved)
      }
      setRefreshVersion((current) => current + 1)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("修改企业成员账号状态失败", {
        user_id: changingUserStatus.id,
        error,
      })
      toast.error(t("members.status.error"))
    } finally {
      setDeleting(false)
    }
  }

  /** 禁用 AI 员工账号或恢复为正常状态。 */
  async function changeAgentStatus() {
    if (!changingAgentStatus) return
    setDeleting(true)
    try {
      const saved =
        changingAgentStatus.status === UserStatus.UserStatusActive
          ? await deactivateAgent(changingAgentStatus.id)
          : await reactivateAgent(changingAgentStatus.id)
      console.info("AI 员工账号状态已修改", {
        identity_id: saved.identityId,
        agent_id: saved.id,
        status: saved.status,
      })
      toast.success(
        t(
          changingAgentStatus.status === UserStatus.UserStatusActive
            ? "agents.status.deactivated"
            : "agents.status.reactivated",
        ),
      )
      setChangingAgentStatus(null)
      if (detailAgent?.id === saved.id) {
        setDetailAgent(saved)
      }
      setRefreshVersion((current) => current + 1)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("修改 AI 员工状态失败", {
        agent_id: changingAgentStatus.id,
        error,
      })
      toast.error(t("agents.status.error"))
    } finally {
      setDeleting(false)
    }
  }

  /** 删除当前团队并刷新团队列表。 */
  async function removeCurrentTeam() {
    if (!deletingTeam) return
    const deletingTeamID = deletingTeam.id
    setDeleting(true)
    try {
      await deleteTeam(deletingTeamID)
      setTeams((current) =>
        current.filter((team) => team.id !== deletingTeamID),
      )
      setDeletingTeam(null)
      toast.success(t("teams.delete.success"))
      if (teamId === deletingTeamID) {
        navigate("/contacts/employees", { replace: true })
      }
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("删除团队失败", error)
      toast.error(t("teams.delete.error"))
    } finally {
      setDeleting(false)
    }
  }

  /** 将选中的团队成员批量移出当前团队。 */
  async function removeMembersFromCurrentTeam() {
    if (!selectedTeam || removingTeamMembers.length === 0) return
    setDeleting(true)
    try {
      const saved = await removeTeamMembers(selectedTeam.id, {
        members: removingTeamMembers.map((member) => ({
          identityType: member.identityType,
          identityId: member.identityId,
        })),
      })
      setTeams((current) =>
        current.map((team) => (team.id === saved.id ? saved : team)),
      )
      toast.success(
        t(
          removingTeamMembers.length === 1
            ? "teams.members.removed"
            : "teams.members.removedMultiple",
          { count: removingTeamMembers.length },
        ),
      )
      setRemovingTeamMembers([])
      setSelectedTeamMemberIdentityIDs(new Set())
      setRefreshVersion((current) => current + 1)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("移出团队成员失败", error)
      toast.error(t("teams.members.removeError"))
    } finally {
      setDeleting(false)
    }
  }

  const hasExternalFilters = Boolean(stage || methodType)
  const roleOptions = roles.map((item) => ({
    value: item.id,
    label: roleDisplayName(item, tCommon),
  }))
  const usesWorkStatusFilter = scope === "team"
  const hasInternalFilters = usesWorkStatusFilter
    ? Boolean(workStatus)
    : Boolean(
        status !== UserStatus.UserStatusActive ||
          (scope === "employees" && roleId),
      )
  const allVisibleTeamMembersSelected =
    teamMembers.length > 0 &&
    teamMembers.every((member) =>
      selectedTeamMemberIdentityIDs.has(member.identityId),
    )
  const currentListEmpty =
    scope === "employees"
      ? users.length === 0
      : scope === "agents"
        ? agents.length === 0
        : scope === "team"
          ? teamMembers.length === 0
          : contacts.length === 0
  const tableColumnCount =
    scope === "employees"
      ? 8
      : scope === "agents"
        ? 6
        : scope === "team"
          ? 6
          : 7

  /** 切换当前页所有团队成员的选中状态。 */
  function toggleAllVisibleTeamMembers(checked: boolean) {
    setSelectedTeamMemberIdentityIDs(
      checked
        ? new Set(teamMembers.map((member) => member.identityId))
        : new Set(),
    )
  }

  /** 切换单个团队成员的选中状态。 */
  function toggleTeamMember(identityID: string, checked: boolean) {
    setSelectedTeamMemberIdentityIDs((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(identityID)
      } else {
        next.delete(identityID)
      }
      return next
    })
  }

  return (
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <ContactScopeSidebar
          scope={scope}
          deleted={deleted}
          channelId={channelId}
          channels={channels}
          teamId={teamId}
          teams={teams}
        />
      }
    >
      <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <PageHeader
          title={title}
          beforeTitle={
            <div className="w-full md:hidden">
              <NativeSelect
                className="h-8 w-full"
                aria-label={t("scopeNavigation")}
                value={mobileScope}
                onChange={(event) => changeMobileScope(event.target.value)}
              >
                <option value="employees">{t("scopes.employees")}</option>
                <option value="agents">{t("scopes.agents")}</option>
                {teams.map((team) => (
                  <option key={team.id} value={`team:${team.id}`}>
                    {t("scopes.teams")} · {team.name}
                  </option>
                ))}
                <option value="external">
                  {t("scopes.external")} · {t("all")}
                </option>
                {channels.map((channel) => (
                  <option key={channel.id} value={`channel:${channel.id}`}>
                    {t("scopes.external")} · {channel.name}
                  </option>
                ))}
              </NativeSelect>
            </div>
          }
        >
          {selectedTeam ? (
            <>
              {selectedTeamMemberIdentityIDs.size > 0 ? (
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() =>
                    setRemovingTeamMembers(
                      teamMembers.filter((member) =>
                        selectedTeamMemberIdentityIDs.has(member.identityId),
                      ),
                    )
                  }
                >
                  {t("teams.members.removeSelected", {
                    count: selectedTeamMemberIdentityIDs.size,
                  })}
                </Button>
              ) : null}
              <Button
                size="sm"
                onClick={() => setParameters({ addMembers: "1" })}
              >
                {t("teams.members.add")}
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t("teams.more")}
                    title={t("teams.more")}
                  >
                    <MoreHorizontalIcon />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    onSelect={() => setParameters({ editTeam: "1" })}
                  >
                    {t("teams.edit")}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    destructive
                    onSelect={() => setDeletingTeam(selectedTeam)}
                  >
                    {t("teams.delete.action")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          ) : null}
        </PageHeader>

        <ListToolbar>
          <ListToolbarSearch
            value={search}
            aria-label={
              scope === "employees"
                ? t("search.employees")
                : scope === "agents"
                  ? t("search.agents")
                  : scope === "team"
                    ? t("search.teamMembers")
                    : t("search.external")
            }
            placeholder={
              scope === "external" ? t("search.external") : undefined
            }
            onChange={(event) => setSearch(event.target.value)}
          />
          {scope === "employees" || scope === "agents" ? (
            <>
              <ListToolbarFilter
                label={t("filters.accountStatus")}
                value={status}
                options={[
                  {
                    value: UserStatus.UserStatusActive,
                    label: t("statuses.active"),
                  },
                  {
                    value: UserStatus.UserStatusInactive,
                    label: t("statuses.inactive"),
                  },
                ]}
                onValueChange={(value) =>
                  setParameters({
                    status:
                      value === UserStatus.UserStatusActive ? null : value,
                    page: null,
                    selected: null,
                  })
                }
              />
              {scope === "employees" ? (
                <ListToolbarFilter
                  label={t("filters.role")}
                  allLabel={t("filters.allRoles")}
                  value={roleId}
                  options={roleOptions}
                  contentClassName="max-h-[min(18rem,var(--radix-dropdown-menu-content-available-height))]"
                  onValueChange={(value) =>
                    setParameters({
                      roleId: value || null,
                      page: null,
                      selected: null,
                    })
                  }
                />
              ) : null}
              {hasInternalFilters ? (
                <ListToolbarReset
                  onClick={() =>
                    setParameters({
                      status: null,
                      roleId: null,
                      page: null,
                    })
                  }
                >
                  {t("filters.clear")}
                </ListToolbarReset>
              ) : null}
            </>
          ) : null}
          {usesWorkStatusFilter ? (
            <>
              <ListToolbarFilter
                label={t("filters.workStatus")}
                allLabel={t("filters.allWorkStatuses")}
                value={workStatus ?? ""}
                options={[
                  {
                    value: WorkStatus.WorkStatusWorking,
                    label: workStatusLabel(
                      WorkStatus.WorkStatusWorking,
                      tCommon,
                    ),
                  },
                  {
                    value: WorkStatus.WorkStatusAway,
                    label: workStatusLabel(WorkStatus.WorkStatusAway, tCommon),
                  },
                  {
                    value: WorkStatus.WorkStatusOffDuty,
                    label: workStatusLabel(
                      WorkStatus.WorkStatusOffDuty,
                      tCommon,
                    ),
                  },
                ]}
                onValueChange={(value) =>
                  setParameters({
                    workStatus: value || null,
                    page: null,
                    selected: null,
                  })
                }
              />
              {hasInternalFilters ? (
                <ListToolbarReset
                  onClick={() =>
                    setParameters({
                      workStatus: null,
                      page: null,
                    })
                  }
                >
                  {t("filters.clear")}
                </ListToolbarReset>
              ) : null}
            </>
          ) : null}
          {scope === "external" ? (
            <ListToolbarFilter
              label={t("filters.view")}
              value={deleted ? "trash" : "active"}
              options={[
                { value: "active", label: t("filters.activeContacts") },
                { value: "trash", label: t("trash.title") },
              ]}
              onValueChange={(value) =>
                setParameters({
                  view: value === "trash" ? "trash" : null,
                  stage: null,
                  methodType: null,
                  page: null,
                  selected: null,
                })
              }
            />
          ) : null}
          {scope === "external" && !deleted ? (
            <>
              <ListToolbarFilter
                label={t("filters.stage")}
                allLabel={t("filters.allStages")}
                value={stage ?? ""}
                options={[
                  {
                    value: ContactStage.ContactStageVisitor,
                    label: t("stages.visitor"),
                  },
                  {
                    value: ContactStage.ContactStageLead,
                    label: t("stages.lead"),
                  },
                  {
                    value: ContactStage.ContactStageCustomer,
                    label: t("stages.customer"),
                  },
                ]}
                onValueChange={(value) =>
                  setParameters({
                    stage: value || null,
                    page: null,
                    selected: null,
                  })
                }
              />
              <ListToolbarFilter
                label={t("filters.method")}
                allLabel={t("filters.allMethods")}
                value={methodType ?? ""}
                options={[
                  {
                    value: ContactMethodType.ContactMethodTypeEmail,
                    label: t("methods.email"),
                  },
                  {
                    value: ContactMethodType.ContactMethodTypePhone,
                    label: t("methods.phone"),
                  },
                ]}
                onValueChange={(value) =>
                  setParameters({
                    methodType: value || null,
                    page: null,
                    selected: null,
                  })
                }
              />
              {hasExternalFilters ? (
                <ListToolbarReset
                  onClick={() =>
                    setParameters({
                      stage: null,
                      methodType: null,
                      page: null,
                    })
                  }
                >
                  {t("filters.clear")}
                </ListToolbarReset>
              ) : null}
            </>
          ) : null}
          {scope === "external" ? (
            <div className="ml-auto">
              <ListToolbarFilter
                label={t("filters.sort")}
                value={sort}
                align="end"
                options={[
                  {
                    value: ContactSort.ContactSortCreatedAtDescending,
                    label: t("sort.created"),
                  },
                  {
                    value: ContactSort.ContactSortUpdatedAtDescending,
                    label: t("sort.updated"),
                  },
                  {
                    value: ContactSort.ContactSortDisplayNameAscending,
                    label: t("sort.name"),
                  },
                ]}
                onValueChange={(value) =>
                  setParameters({ sort: value, page: null, selected: null })
                }
              />
            </div>
          ) : null}
        </ListToolbar>

        <PageContent>
          {loadState === "loading" ? (
            <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
              <LoaderCircleIcon className="size-4 animate-spin" />
              {t("loading")}
            </div>
          ) : loadState === "error" ? (
            <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
              <p className="text-sm text-muted-foreground">
                {t("list.loadError")}
              </p>
              <Button
                className="mt-4"
                variant="outline"
                onClick={() => void loadList()}
              >
                {t("retry")}
              </Button>
            </div>
          ) : (
            <div className="overflow-hidden rounded-lg border bg-card">
              <Table>
                <TableHeader>
                  {scope === "employees" ? (
                    <TableRow className="hover:bg-transparent">
                      <TableHead>{t("columns.employeeName")}</TableHead>
                      <TableHead>{t("columns.email")}</TableHead>
                      <TableHead>{t("columns.joinedTeams")}</TableHead>
                      <TableHead>{t("columns.role")}</TableHead>
                      <TableHead>{t("columns.accountStatus")}</TableHead>
                      <TableHead>{t("columns.workStatus")}</TableHead>
                      <TableHead>{t("columns.createdAt")}</TableHead>
                      <TableHead className="text-right">
                        {t("columns.actions")}
                      </TableHead>
                    </TableRow>
                  ) : scope === "agents" ? (
                    <TableRow className="hover:bg-transparent">
                      <TableHead>{t("columns.name")}</TableHead>
                      <TableHead>{t("columns.joinedTeams")}</TableHead>
                      <TableHead>{t("columns.accountStatus")}</TableHead>
                      <TableHead>{t("columns.workStatus")}</TableHead>
                      <TableHead>{t("columns.createdAt")}</TableHead>
                      <TableHead className="text-right">
                        {t("columns.actions")}
                      </TableHead>
                    </TableRow>
                  ) : scope === "team" ? (
                    <TableRow className="hover:bg-transparent">
                      <TableHead className="w-10">
                        <input
                          type="checkbox"
                          className="size-4 accent-primary"
                          aria-label={t("teams.members.selectAll")}
                          checked={allVisibleTeamMembersSelected}
                          onChange={(event) =>
                            toggleAllVisibleTeamMembers(event.target.checked)
                          }
                        />
                      </TableHead>
                      <TableHead>{t("columns.memberName")}</TableHead>
                      <TableHead>{t("columns.type")}</TableHead>
                      <TableHead>{t("columns.workStatus")}</TableHead>
                      <TableHead>{t("columns.joinedAt")}</TableHead>
                      <TableHead className="text-right">
                        {t("columns.actions")}
                      </TableHead>
                    </TableRow>
                  ) : (
                    <TableRow className="hover:bg-transparent">
                      <TableHead>{t("columns.name")}</TableHead>
                      <TableHead>{t("columns.stage")}</TableHead>
                      <TableHead>{t("columns.email")}</TableHead>
                      <TableHead>{t("columns.phone")}</TableHead>
                      <TableHead>{t("columns.channels")}</TableHead>
                      <TableHead>
                        {deleted
                          ? t("columns.deletedAt")
                          : t("columns.addedAt")}
                      </TableHead>
                      <TableHead className="text-right">
                        {t("columns.actions")}
                      </TableHead>
                    </TableRow>
                  )}
                </TableHeader>
                <TableBody>
                  {scope === "employees" && users.length > 0
                    ? users.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell className="font-medium">
                            {user.displayName}
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {user.email}
                          </TableCell>
                          <TableCell className="max-w-xs">
                            <JoinedTeamsCell teams={user.teams} />
                          </TableCell>
                          <TableCell>
                            {roleDisplayName(user.role, tCommon)}
                          </TableCell>
                          <TableCell>
                            <UserStatusBadge
                              status={user.status}
                              label={userStatusLabel(user.status, t)}
                            />
                          </TableCell>
                          <TableCell>
                            <WorkStatusBadge status={memberWorkStatus(user)} />
                          </TableCell>
                          <TableCell className="whitespace-nowrap text-muted-foreground">
                            {formatDateTime(user.createdAt)}
                          </TableCell>
                          <TableCell className="text-right whitespace-nowrap">
                            <div className="flex justify-end gap-2">
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() =>
                                  setParameters({ selected: user.id })
                                }
                              >
                                {t("detail.action")}
                              </Button>
                              <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                  <Button
                                    variant="ghost"
                                    size="icon-sm"
                                    aria-label={t("list.more")}
                                    title={t("list.more")}
                                  >
                                    <MoreHorizontalIcon />
                                  </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end">
                                  <DropdownMenuItem
                                    destructive={
                                      user.status === UserStatus.UserStatusActive
                                    }
                                    onSelect={() =>
                                      setChangingUserStatus(user)
                                    }
                                  >
                                    {t(
                                      user.status ===
                                        UserStatus.UserStatusActive
                                        ? "members.status.deactivate"
                                        : "members.status.reactivate",
                                    )}
                                  </DropdownMenuItem>
                                </DropdownMenuContent>
                              </DropdownMenu>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))
                    : null}
                  {scope === "agents" && agents.length > 0
                    ? agents.map((agent) => (
                        <TableRow key={agent.id}>
                          <TableCell className="font-medium">
                            {agent.displayName}
                          </TableCell>
                          <TableCell className="max-w-xs">
                            <JoinedTeamsCell teams={agent.teams} />
                          </TableCell>
                          <TableCell>
                            <UserStatusBadge
                              status={agent.status}
                              label={userStatusLabel(agent.status, t)}
                            />
                          </TableCell>
                          <TableCell>
                            <WorkStatusBadge status={agent.workStatus} />
                          </TableCell>
                          <TableCell className="whitespace-nowrap text-muted-foreground">
                            {formatDateTime(agent.createdAt)}
                          </TableCell>
                          <TableCell className="text-right whitespace-nowrap">
                            <div className="flex justify-end gap-2">
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() =>
                                  setParameters({ selected: agent.id })
                                }
                              >
                                {t("detail.action")}
                              </Button>
                              <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                  <Button
                                    variant="ghost"
                                    size="icon-sm"
                                    aria-label={t("list.more")}
                                    title={t("list.more")}
                                  >
                                    <MoreHorizontalIcon />
                                  </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end">
                                  <DropdownMenuItem
                                    destructive={
                                      agent.status ===
                                      UserStatus.UserStatusActive
                                    }
                                    onSelect={() =>
                                      setChangingAgentStatus(agent)
                                    }
                                  >
                                    {t(
                                      agent.status ===
                                        UserStatus.UserStatusActive
                                        ? "agents.status.deactivate"
                                        : "agents.status.reactivate",
                                    )}
                                  </DropdownMenuItem>
                                </DropdownMenuContent>
                              </DropdownMenu>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))
                    : null}
                  {scope === "team" && teamMembers.length > 0
                    ? teamMembers.map((member) => (
                        <TableRow key={member.identityId}>
                          <TableCell>
                            <input
                              type="checkbox"
                              className="size-4 accent-primary"
                              aria-label={t("teams.members.selectMember", {
                                name: member.displayName,
                              })}
                              checked={selectedTeamMemberIdentityIDs.has(
                                member.identityId,
                              )}
                              onChange={(event) =>
                                toggleTeamMember(
                                  member.identityId,
                                  event.target.checked,
                                )
                              }
                            />
                          </TableCell>
                          <TableCell className="font-medium">
                            {member.displayName}
                          </TableCell>
                          <TableCell>
                            {t(
                              member.identityType ===
                                OrganizationIdentityType.OrganizationIdentityTypeAgent
                                ? "identityCategories.agent"
                                : "identityCategories.user",
                            )}
                          </TableCell>
                          <TableCell>
                            <WorkStatusBadge
                              status={identityWorkStatus(member)}
                            />
                          </TableCell>
                          <TableCell className="whitespace-nowrap text-muted-foreground">
                            {formatDateTime(member.joinedAt)}
                          </TableCell>
                          <TableCell className="text-right whitespace-nowrap">
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  aria-label={t("list.more")}
                                  title={t("list.more")}
                                >
                                  <MoreHorizontalIcon />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuItem
                                  destructive
                                  onSelect={() =>
                                    setRemovingTeamMembers([member])
                                  }
                                >
                                  {t("teams.members.remove")}
                                </DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </TableCell>
                        </TableRow>
                      ))
                    : null}
                  {scope === "external" && contacts.length > 0
                    ? contacts.map((contact) => (
                        <TableRow key={contact.id}>
                          <TableCell className="font-medium">
                            {contact.displayName || t("anonymous")}
                          </TableCell>
                          <TableCell>
                            <StageLabel stage={contact.stage} />
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {contact.primaryEmail || "—"}
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {contact.primaryPhone || "—"}
                          </TableCell>
                          <TableCell>{contact.sourceChannelName}</TableCell>
                          <TableCell className="whitespace-nowrap text-muted-foreground">
                            {formatDateTime(
                              deleted && contact.deletedAt
                                ? contact.deletedAt
                                : contact.createdAt,
                            )}
                          </TableCell>
                          {deleted ? (
                            <TableCell className="text-right whitespace-nowrap">
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setRestoringContact(contact)}
                              >
                                {t("trash.restore")}
                              </Button>
                            </TableCell>
                          ) : (
                            <TableCell className="text-right whitespace-nowrap">
                              <div className="flex justify-end gap-2">
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() =>
                                    setParameters({ selected: contact.id })
                                  }
                                >
                                  {t("detail.action")}
                                </Button>
                                <DropdownMenu>
                                  <DropdownMenuTrigger asChild>
                                    <Button
                                      variant="ghost"
                                      size="icon-sm"
                                      aria-label={t("list.more")}
                                      title={t("list.more")}
                                    >
                                      <MoreHorizontalIcon />
                                    </Button>
                                  </DropdownMenuTrigger>
                                  <DropdownMenuContent align="end">
                                    <DropdownMenuItem
                                      destructive
                                      onSelect={() =>
                                        setDeletingContact(contact)
                                      }
                                    >
                                      {t("delete.action")}
                                    </DropdownMenuItem>
                                  </DropdownMenuContent>
                                </DropdownMenu>
                              </div>
                            </TableCell>
                          )}
                        </TableRow>
                      ))
                    : null}
                  {currentListEmpty ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell
                        colSpan={tableColumnCount}
                        className="h-32 text-center text-muted-foreground"
                      >
                        {deleted ? t("trash.empty") : t("list.empty")}
                      </TableCell>
                    </TableRow>
                  ) : null}
                </TableBody>
              </Table>
              <PageControls
                page={page}
                onPageChange={(number) =>
                  setParameters({ page: String(number), selected: null })
                }
              />
            </div>
          )}
        </PageContent>
      </section>

      <Sheet
        open={Boolean(selected)}
        onOpenChange={(open) => !open && closeDetail()}
      >
        <SheetContent
          className="w-full gap-0 p-0 sm:max-w-xl"
          onOpenAutoFocus={(event) => {
            event.preventDefault()
            detailTitleRef.current?.focus()
          }}
        >
          <SheetHeader className="border-b px-6 py-4 pr-12">
            <SheetTitle
              ref={detailTitleRef}
              tabIndex={-1}
              className="outline-none"
            >
              {scope === "employees"
                ? (detailUser?.displayName ?? t("detail.memberTitle"))
                : scope === "agents"
                  ? (detailAgent?.displayName ?? t("detail.agentTitle"))
                : detail?.contact.displayName || t("anonymous")}
            </SheetTitle>
            <SheetDescription>
              {scope === "employees"
                ? t("detail.memberDescription")
                : scope === "agents"
                  ? t("detail.agentDescription")
                : t("detail.contactDescription")}
            </SheetDescription>
          </SheetHeader>
          <ScrollArea className="min-h-0 flex-1">
            <div className="p-6">
              {detailLoading ? (
                <div className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground">
                  <LoaderCircleIcon className="size-4 animate-spin" />
                  {t("loading")}
                </div>
              ) : scope === "employees" && detailUser ? (
                <MemberDetailView
                  key={detailUser.id}
                  user={detailUser}
                  teams={teams}
                  roles={roles}
                  workStatus={memberWorkStatus(detailUser)}
                  onSaved={(saved) => {
                    setDetailUser(saved)
                    setRefreshVersion((current) => current + 1)
                    if (saved.id === identity.user.id) {
                      updateWorkspaceUser({
                        ...identity.user,
                        displayName: saved.displayName,
                        email: saved.email,
                        roleId: saved.role.id,
                        status: saved.status,
                      })
                    }
                  }}
                  onNotFound={refreshAndClose}
                />
              ) : scope === "agents" && detailAgent ? (
                <AgentDetailView
                  key={detailAgent.id}
                  agent={detailAgent}
                  teams={teams}
                  onSaved={(saved) => {
                    setDetailAgent(saved)
                    setRefreshVersion((current) => current + 1)
                  }}
                  onNotFound={refreshAndClose}
                />
              ) : scope === "external" && detail ? (
                <ContactDetailView
                  detail={detail}
                  onSaved={(saved) => {
                    setDetail(saved)
                    setRefreshVersion((current) => current + 1)
                  }}
                  onNotFound={refreshAndClose}
                />
              ) : null}
            </div>
          </ScrollArea>
        </SheetContent>
      </Sheet>

      <Dialog
        open={creating}
        onOpenChange={(open) => !open && setParameters({ new: null })}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {t(scope !== "external" ? "members.create" : "detail.createTitle")}
            </DialogTitle>
            <DialogDescription>
              {t(
                scope !== "external"
                  ? "members.createDescription"
                  : "detail.createDescription",
              )}
            </DialogDescription>
          </DialogHeader>
          {scope !== "external" ? (
            <MemberForm
              teams={teams}
              roles={roles}
              defaultTeamIds={selectedTeam ? [selectedTeam.id] : []}
              onSaved={() => {
                setParameters({ new: null })
                setRefreshVersion((current) => current + 1)
              }}
              onCancel={() => setParameters({ new: null })}
            />
          ) : (
            <ContactForm
              channels={channels}
              onSaved={() => {
                setParameters({ new: null })
                setRefreshVersion((current) => current + 1)
              }}
              onCancel={() => setParameters({ new: null })}
            />
          )}
        </DialogContent>
      </Dialog>

      <Dialog
        open={creatingAgent}
        onOpenChange={(open) => !open && setParameters({ newAgent: null })}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("agents.create")}</DialogTitle>
            <DialogDescription>
              {t("agents.createDescription")}
            </DialogDescription>
          </DialogHeader>
          <AgentForm
            teams={teams}
            defaultTeamIds={selectedTeam ? [selectedTeam.id] : []}
            onSaved={() => {
              setParameters({ newAgent: null })
              setRefreshVersion((current) => current + 1)
            }}
            onCancel={() => setParameters({ newAgent: null })}
          />
        </DialogContent>
      </Dialog>

      <Dialog
        open={creatingTeam}
        onOpenChange={(open) => !open && setParameters({ newTeam: null })}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("teams.create")}</DialogTitle>
            <DialogDescription>{t("teams.editDescription")}</DialogDescription>
          </DialogHeader>
          <TeamForm
            onSaved={(team) => {
              setTeams((current) => [...current, team])
              navigate(`/contacts/teams/${team.id}`, {
                replace: true,
              })
            }}
            onCancel={() => setParameters({ newTeam: null })}
          />
        </DialogContent>
      </Dialog>

      {selectedTeam ? (
        <Dialog
          open={editingTeam}
          onOpenChange={(open) => !open && setParameters({ editTeam: null })}
        >
          <DialogContent className="max-w-xl">
            <DialogHeader>
              <DialogTitle>{t("teams.edit")}</DialogTitle>
              <DialogDescription>
                {t("teams.createDescription")}
              </DialogDescription>
            </DialogHeader>
            <TeamForm
              team={selectedTeam}
              onSaved={(saved) => {
                setTeams((current) =>
                  current.map((team) => (team.id === saved.id ? saved : team)),
                )
                setParameters({ editTeam: null })
              }}
              onCancel={() => setParameters({ editTeam: null })}
            />
          </DialogContent>
        </Dialog>
      ) : null}

      {selectedTeam ? (
        <Dialog
          open={addingTeamMembers}
          onOpenChange={(open) => !open && setParameters({ addMembers: null })}
        >
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>{t("teams.members.add")}</DialogTitle>
              <DialogDescription>
                {t("teams.members.addDescription", {
                  name: selectedTeam.name,
                })}
              </DialogDescription>
            </DialogHeader>
            <TeamMemberPicker
              team={selectedTeam}
              onSaved={(saved) => {
                setTeams((current) =>
                  current.map((team) => (team.id === saved.id ? saved : team)),
                )
                setParameters({ addMembers: null })
                setRefreshVersion((current) => current + 1)
              }}
              onCancel={() => setParameters({ addMembers: null })}
            />
          </DialogContent>
        </Dialog>
      ) : null}

      <AlertDialog
        open={deletingTeam !== null}
        onOpenChange={(open) => !open && setDeletingTeam(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("teams.delete.title", { name: deletingTeam?.name ?? "" })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("teams.delete.description", {
                count: deletingTeam?.memberCount ?? 0,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("teams.form.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void removeCurrentTeam()}
            >
              {t("teams.delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={removingTeamMembers.length > 0}
        onOpenChange={(open) => !open && setRemovingTeamMembers([])}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {removingTeamMembers.length === 1
                ? t("teams.members.removeTitle", {
                    name: removingTeamMembers[0].displayName,
                  })
                : t("teams.members.removeMultipleTitle", {
                    count: removingTeamMembers.length,
                  })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                removingTeamMembers.length === 1
                  ? "teams.members.removeDescription"
                  : "teams.members.removeMultipleDescription",
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("teams.form.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void removeMembersFromCurrentTeam()}
            >
              {t("teams.members.remove")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={changingUserStatus !== null}
        onOpenChange={(open) => !open && setChangingUserStatus(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(
                changingUserStatus?.status === UserStatus.UserStatusActive
                  ? "members.status.deactivateTitle"
                  : "members.status.reactivateTitle",
                { name: changingUserStatus?.displayName ?? "" },
              )}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                changingUserStatus?.status === UserStatus.UserStatusActive
                  ? "members.status.deactivateDescription"
                  : "members.status.reactivateDescription",
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("members.status.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void changeUserStatus()}
            >
              {deleting
                ? t("members.status.saving")
                : t("members.status.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={changingAgentStatus !== null}
        onOpenChange={(open) => !open && setChangingAgentStatus(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(
                changingAgentStatus?.status === UserStatus.UserStatusActive
                  ? "agents.status.deactivateTitle"
                  : "agents.status.reactivateTitle",
                { name: changingAgentStatus?.displayName ?? "" },
              )}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                changingAgentStatus?.status === UserStatus.UserStatusActive
                  ? "agents.status.deactivateDescription"
                  : "agents.status.reactivateDescription",
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("agents.status.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void changeAgentStatus()}
            >
              {deleting
                ? t("agents.status.saving")
                : t("agents.status.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={deletingContact !== null}
        onOpenChange={(open) => !open && setDeletingContact(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("delete.title", {
                name: deletingContact?.displayName || t("anonymous"),
              })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("delete.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("delete.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void removeContact()}
            >
              {deleting ? t("delete.deleting") : t("delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={restoringContact !== null}
        onOpenChange={(open) => !open && setRestoringContact(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("trash.restoreTitle", {
                name: restoringContact?.displayName || t("anonymous"),
              })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("trash.restoreDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("trash.restoreCancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void restore()}
            >
              {deleting ? t("trash.restoring") : t("trash.restoreConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageSplit>
  )
}
