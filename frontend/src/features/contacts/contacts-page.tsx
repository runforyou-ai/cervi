/** 通讯录列表、筛选、详情和回收站。 */
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react"
import {
  BotIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ContactRoundIcon,
  GlobeIcon,
  LoaderCircleIcon,
  MoreHorizontalIcon,
  UsersIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearchParams } from "react-router"
import { toast } from "sonner"

import {
  ChannelType,
  ContactMethodType,
  ContactSort,
  ContactStage,
  UserRole,
  UserStatus,
  deleteContact,
  getContact,
  getUser,
  listChannels,
  listContacts,
  listDeletedContacts,
  recoverSession,
  listUsers,
  restoreContact,
  type ChannelSummary,
  type ContactDetail,
  type ContactListResponse,
  type ContactSummary,
  type DirectoryUser,
  type PageInfo,
} from "@/api"
import { optionalWailsEnum } from "@/lib/wails-enum"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { PageBack } from "@/components/page-back"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { PagePaneNav, PageSplit } from "@/components/page-split"
import { SelectableText } from "@/components/selectable-text"
import { StatusBadge } from "@/components/status-badge"
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
import { ContactForm } from "@/features/contacts/contact-form"
import { ContactDetailView } from "@/features/contacts/contact-detail"
import {
  channelTypeLabel,
  userRoleLabel,
  userStatusLabel,
} from "@/features/contacts/contact-labels"
import { useDateTime } from "@/hooks/use-date-time"
import { cn } from "@/lib/utils"

export type ContactScope = "internal" | "external" | "agents"

type LoadState = "loading" | "ready" | "error"

const contactNavHoverClass =
  "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
const contactNavLeafActiveClass =
  "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
const contactNavPathActiveClass =
  "font-medium text-sidebar-accent-foreground"
const contactNavSubitemClass =
  "flex h-8 w-full items-center gap-2 rounded-md py-1.5 pr-2 text-left text-sm text-muted-foreground transition-colors"

/** 通讯录分类按钮。 */
function ScopeButton({
  active,
  icon: Icon,
  children,
  onClick,
}: {
  active: boolean
  icon: typeof UsersIcon
  children: React.ReactNode
  onClick: () => void
}) {
  return (
    <button
      type="button"
      className={cn(
        "flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors",
        contactNavHoverClass,
        active && contactNavLeafActiveClass,
      )}
      onClick={onClick}
    >
      <Icon className="size-4 shrink-0" />
      <span className="truncate">{children}</span>
    </button>
  )
}

/** 通讯录子分类按钮。 */
function SubscopeButton({
  active,
  children,
  nested = false,
  onClick,
  icon: Icon,
}: {
  active: boolean
  children: React.ReactNode
  nested?: boolean
  onClick: () => void
  icon?: typeof GlobeIcon
}) {
  return (
    <button
      type="button"
      className={cn(
        contactNavSubitemClass,
        contactNavHoverClass,
        nested ? "pl-14" : "pl-8",
        active && contactNavLeafActiveClass,
      )}
      onClick={onClick}
    >
      {Icon ? <Icon className="size-3.5 shrink-0" /> : null}
      <span className="truncate">{children}</span>
    </button>
  )
}

/** 通讯录分类和来源渠道筛选。 */
function ContactScopeSidebar({
  scope,
  deleted,
  channelId,
  channels,
}: {
  scope: ContactScope
  deleted: boolean
  channelId: string
  channels: ChannelSummary[]
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
    <PagePaneNav label={t("scopeNavigation")} title={t("title")}>
      <ScopeButton
        active={scope === "internal"}
        icon={UsersIcon}
        onClick={() => navigate("/contacts/internal")}
      >
        {t("scopes.internal")}
      </ScopeButton>

      <ScopeButton
        active={scope === "agents"}
        icon={BotIcon}
        onClick={() => navigate("/contacts/agents")}
      >
        {t("scopes.agents")}
      </ScopeButton>

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
            <ChevronRightIcon className="ml-auto size-4 transition-transform group-data-[state=open]:rotate-90" />
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
                  <ChevronRightIcon className="size-3.5 transition-transform group-data-[state=open]:rotate-90" />
                  <span>{channelTypeLabel(type, t)}</span>
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

/** 显示成员状态徽标。 */
function UserStatusBadge({
  status,
  label,
}: {
  status: UserStatus
  label: string
}) {
  const active = status === UserStatus.UserStatusActive

  return (
    <StatusBadge variant={active ? "success" : "muted"}>{label}</StatusBadge>
  )
}

/** 显示团队成员详情中的一个只读字段。 */
function MemberDetailItem({
  label,
  value,
  emphasized = false,
}: {
  label: string
  value: React.ReactNode
  emphasized?: boolean
}) {
  return (
    <div>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={cn("mt-1", emphasized && "font-medium")}>
        {typeof value === "string" ? (
          <SelectableText>{value}</SelectableText>
        ) : (
          value
        )}
      </dd>
    </div>
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
        <span>{t("pagination.page", { current: page.number, total: totalPages })}</span>
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
export function ContactsPage({
  scope,
  deleted = false,
}: {
  scope: ContactScope
  deleted?: boolean
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get("q") ?? ""
  const [search, setSearch] = useState(query)
  const [channels, setChannels] = useState<ChannelSummary[]>([])
  const [contacts, setContacts] = useState<ContactSummary[]>([])
  const [users, setUsers] = useState<DirectoryUser[]>([])
  const [page, setPage] = useState<PageInfo>({ number: 1, size: 50, total: 0 })
  const [loadState, setLoadState] = useState<LoadState>("loading")
  const [refreshVersion, setRefreshVersion] = useState(0)
  const [detail, setDetail] = useState<ContactDetail | null>(null)
  const [detailUser, setDetailUser] = useState<DirectoryUser | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [deletingContact, setDeletingContact] = useState<ContactSummary | null>(null)
  const [deleting, setDeleting] = useState(false)
  const detailTitleRef = useRef<HTMLHeadingElement>(null)
  const selected = searchParams.get("selected") ?? ""
  const creating = searchParams.get("new") === "1"
  const channelId = searchParams.get("channelId") ?? ""
  const stage = optionalWailsEnum(ContactStage, searchParams.get("stage"))
  const methodType = optionalWailsEnum(
    ContactMethodType,
    searchParams.get("methodType"),
  )
  const sort =
    optionalWailsEnum(ContactSort, searchParams.get("sort")) ??
    ContactSort.ContactSortCreatedAtDescending
  const status = optionalWailsEnum(UserStatus, searchParams.get("status"))
  const role = optionalWailsEnum(UserRole, searchParams.get("role"))
  const currentPage = Number(searchParams.get("page") ?? "1") || 1

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
    const timeout = window.setTimeout(() => {
      if (search !== query) {
        setParameters({ q: search || null, page: null, selected: null })
      }
    }, 300)
    return () => window.clearTimeout(timeout)
  }, [query, search, setParameters])

  useEffect(() => {
    const controller = new AbortController()
    void listChannels(controller.signal)
      .then(setChannels)
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return
        }
        console.warn("渠道列表加载失败", error)
        setChannels([])
      })
    return () => controller.abort()
  }, [])

  /** 按当前范围加载联系人或企业成员列表。 */
  const loadList = useCallback(
    async (signal?: AbortSignal) => {
      setLoadState("loading")
      try {
        if (scope === "external") {
          const loader = deleted ? listDeletedContacts : listContacts
          const response: ContactListResponse = await loader(
            {
              query,
              stage,
              channelId: deleted ? "" : channelId,
              methodType,
              sort,
              page: currentPage,
              pageSize: 50,
            },
            signal,
          )
          setContacts(response.contacts)
          setPage(response.page)
        } else if (scope === "internal") {
          const response = await listUsers(
            { query, status, role, page: currentPage, pageSize: 50 },
            signal,
          )
          setUsers(response.users)
          setPage(response.page)
        } else {
          setPage({ number: 1, size: 50, total: 0 })
        }
        setLoadState("ready")
      } catch (error) {
        if (error instanceof DOMException && error.name === "AbortError") {
          return
        }
        if (recoverSession(error, navigate)) {
          return
        }
        console.warn("联系人列表加载失败", error)
        setLoadState("error")
      }
    },
    [channelId, currentPage, deleted, methodType, navigate, query, role, scope, sort, stage, status],
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadList(controller.signal)
    return () => controller.abort()
  }, [loadList, refreshVersion])

  useEffect(() => {
    setDetail(null)
    setDetailUser(null)
    if (!selected) {
      return
    }
    const controller = new AbortController()
    setDetailLoading(true)
    const loader = scope === "external"
      ? getContact(selected, controller.signal).then(setDetail)
      : scope === "internal"
        ? getUser(selected, controller.signal).then(setDetailUser)
        : Promise.resolve()
    void loader
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return
        }
        if (recoverSession(error, navigate)) {
          return
        }
        console.warn("联系人详情加载失败", error)
        toast.error(t("detail.loadError"))
        setParameters({ selected: null })
      })
      .finally(() => setDetailLoading(false))
    return () => controller.abort()
  }, [navigate, scope, selected, setParameters, t])

  const selectedChannel = channels.find((channel) => channel.id === channelId)
  const title = scope === "internal"
    ? t("scopes.internal")
    : scope === "agents"
      ? t("scopes.agents")
      : deleted
        ? t("trash.title")
        : selectedChannel?.name ?? t("scopes.external")
  const mobileScope = scope === "internal"
    ? "internal"
    : scope === "agents"
      ? "agents"
      : deleted
        ? "trash"
        : channelId
          ? `channel:${channelId}`
          : "external"

  /** 窄视口下切换联系人范围。 */
  function changeMobileScope(value: string) {
    if (value === "internal") {
      navigate("/contacts/internal")
    } else if (value === "agents") {
      navigate("/contacts/agents")
    } else if (value === "trash") {
      navigate("/contacts/external/trash")
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
  async function restore(item: ContactSummary) {
    try {
      await restoreContact(item.id)
      console.info("联系人已恢复", { contact_id: item.id })
      toast.success(t("trash.restored"))
      setRefreshVersion((current) => current + 1)
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("恢复联系人失败", error)
      toast.error(t("trash.restoreError"))
    }
  }

  const hasExternalFilters = Boolean(stage || methodType)
  const hasInternalFilters = Boolean(status || role)

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
        />
      }
    >
      <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <PageHeader
          title={title}
          beforeTitle={(
            <div className="w-full md:hidden">
              <NativeSelect
                className="h-8 w-full"
                aria-label={t("scopeNavigation")}
                value={mobileScope}
                onChange={(event) => changeMobileScope(event.target.value)}
              >
                <option value="internal">{t("scopes.internal")}</option>
                <option value="agents">{t("scopes.agents")}</option>
                <option value="external">{t("scopes.external")} · {t("all")}</option>
                {channels.map((channel) => (
                  <option key={channel.id} value={`channel:${channel.id}`}>
                    {t("scopes.external")} · {channel.name}
                  </option>
                ))}
                <option value="trash">{t("trash.title")}</option>
              </NativeSelect>
            </div>
          )}
        >
          {deleted ? (
            <PageBack to="/contacts/external" />
          ) : scope === "external" ? (
            <>
              <Button
                size="sm"
                disabled={channels.length === 0}
                title={channels.length === 0 ? t("form.channelRequiredHint") : undefined}
                onClick={() => setParameters({ new: "1", selected: null })}
              >
                {t("create")}
              </Button>
              <Button variant="outline" size="sm" onClick={() => navigate("/contacts/external/trash")}>
                {t("trash.title")}
              </Button>
            </>
          ) : null}
        </PageHeader>

        {scope !== "agents" ? (
          <ListToolbar>
            <ListToolbarSearch
              value={search}
              aria-label={scope === "internal" ? t("search.internal") : t("search.external")}
              placeholder={scope === "internal" ? t("search.internal") : t("search.external")}
              onChange={(event) => setSearch(event.target.value)}
            />
            {scope === "internal" ? (
              <>
                <ListToolbarFilter
                  label={t("filters.status")}
                  allLabel={t("filters.allStatuses")}
                  value={status ?? ""}
                  options={[
                    { value: UserStatus.UserStatusActive, label: t("statuses.active") },
                    { value: UserStatus.UserStatusInactive, label: t("statuses.inactive") },
                  ]}
                  onValueChange={(value) =>
                    setParameters({ status: value || null, page: null, selected: null })
                  }
                />
                <ListToolbarFilter
                  label={t("filters.role")}
                  allLabel={t("filters.allRoles")}
                  value={role ?? ""}
                  options={[
                    { value: UserRole.UserRoleOwner, label: t("roles.owner") },
                    { value: UserRole.UserRoleMember, label: t("roles.member") },
                  ]}
                  onValueChange={(value) =>
                    setParameters({ role: value || null, page: null, selected: null })
                  }
                />
                {hasInternalFilters ? (
                  <ListToolbarReset
                    onClick={() => setParameters({ status: null, role: null, page: null })}
                  >
                    {t("filters.clear")}
                  </ListToolbarReset>
                ) : null}
              </>
            ) : null}
            {scope === "external" && !deleted ? (
              <>
                <ListToolbarFilter
                  label={t("filters.stage")}
                  allLabel={t("filters.allStages")}
                  value={stage ?? ""}
                  options={[
                    { value: ContactStage.ContactStageVisitor, label: t("stages.visitor") },
                    { value: ContactStage.ContactStageLead, label: t("stages.lead") },
                    { value: ContactStage.ContactStageCustomer, label: t("stages.customer") },
                  ]}
                  onValueChange={(value) =>
                    setParameters({ stage: value || null, page: null, selected: null })
                  }
                />
                <ListToolbarFilter
                  label={t("filters.method")}
                  allLabel={t("filters.allMethods")}
                  value={methodType ?? ""}
                  options={[
                    { value: ContactMethodType.ContactMethodTypeEmail, label: t("methods.email") },
                    { value: ContactMethodType.ContactMethodTypePhone, label: t("methods.phone") },
                  ]}
                  onValueChange={(value) =>
                    setParameters({ methodType: value || null, page: null, selected: null })
                  }
                />
                {hasExternalFilters ? (
                  <ListToolbarReset
                    onClick={() => setParameters({ stage: null, methodType: null, page: null })}
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
                    { value: ContactSort.ContactSortCreatedAtDescending, label: t("sort.created") },
                    { value: ContactSort.ContactSortUpdatedAtDescending, label: t("sort.updated") },
                    { value: ContactSort.ContactSortDisplayNameAscending, label: t("sort.name") },
                  ]}
                  onValueChange={(value) =>
                    setParameters({ sort: value, page: null, selected: null })
                  }
                />
              </div>
            ) : null}
          </ListToolbar>
        ) : null}

        <PageContent>
          {loadState === "loading" ? (
            <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
              <LoaderCircleIcon className="size-4 animate-spin" />
              {t("loading")}
            </div>
          ) : loadState === "error" ? (
            <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
              <p className="text-sm text-muted-foreground">{t("list.loadError")}</p>
              <Button className="mt-4" variant="outline" onClick={() => void loadList()}>
                {t("retry")}
              </Button>
            </div>
          ) : scope === "agents" ? (
            <div className="flex min-h-64 flex-col items-center justify-center rounded-lg border border-dashed text-center">
              <BotIcon className="size-8 text-muted-foreground" />
              <h3 className="mt-4 font-medium">{t("agents.emptyTitle")}</h3>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground">{t("agents.emptyDescription")}</p>
            </div>
          ) : (
            <div className="overflow-hidden rounded-lg border bg-card">
              <Table>
                <TableHeader>
                  {scope === "internal" ? (
                    <TableRow className="hover:bg-transparent">
                      <TableHead>{t("columns.name")}</TableHead>
                      <TableHead>{t("columns.email")}</TableHead>
                      <TableHead>{t("columns.role")}</TableHead>
                      <TableHead>{t("columns.status")}</TableHead>
                      <TableHead className="text-right">{t("columns.actions")}</TableHead>
                    </TableRow>
                  ) : (
                    <TableRow className="hover:bg-transparent">
                      <TableHead>{t("columns.name")}</TableHead>
                      <TableHead>{t("columns.stage")}</TableHead>
                      <TableHead>{t("columns.email")}</TableHead>
                      <TableHead>{t("columns.phone")}</TableHead>
                      <TableHead>{t("columns.channels")}</TableHead>
                      <TableHead>{deleted ? t("columns.deletedAt") : t("columns.addedAt")}</TableHead>
                      <TableHead className="text-right">{t("columns.actions")}</TableHead>
                    </TableRow>
                  )}
                </TableHeader>
                <TableBody>
                  {scope === "internal" && users.length > 0
                    ? users.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell className="font-medium">{user.displayName}</TableCell>
                          <TableCell className="text-muted-foreground">{user.email}</TableCell>
                          <TableCell>{userRoleLabel(user.role, t)}</TableCell>
                          <TableCell>
                            <UserStatusBadge
                              status={user.status}
                              label={userStatusLabel(user.status, t)}
                            />
                          </TableCell>
                          <TableCell className="text-right whitespace-nowrap">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => setParameters({ selected: user.id })}
                            >
                              {t("detail.action")}
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    : null}
                  {scope === "external" && contacts.length > 0
                    ? contacts.map((contact) => (
                        <TableRow key={contact.id}>
                          <TableCell className="font-medium">{contact.displayName || t("anonymous")}</TableCell>
                          <TableCell><StageLabel stage={contact.stage} /></TableCell>
                          <TableCell className="text-muted-foreground">{contact.primaryEmail || "—"}</TableCell>
                          <TableCell className="text-muted-foreground">{contact.primaryPhone || "—"}</TableCell>
                          <TableCell>{contact.sourceChannelName}</TableCell>
                          <TableCell className="whitespace-nowrap text-muted-foreground">
                            {formatDateTime(deleted && contact.deletedAt ? contact.deletedAt : contact.createdAt)}
                          </TableCell>
                          {deleted ? (
                            <TableCell className="text-right whitespace-nowrap">
                              <Button variant="outline" size="sm" onClick={() => void restore(contact)}>
                                {t("trash.restore")}
                              </Button>
                            </TableCell>
                          ) : (
                            <TableCell className="text-right whitespace-nowrap">
                              <div className="flex justify-end gap-2">
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() => setParameters({ selected: contact.id })}
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
                                      onSelect={() => setDeletingContact(contact)}
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
                  {((scope === "internal" && users.length === 0) || (scope === "external" && contacts.length === 0)) ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={scope === "internal" ? 5 : 7} className="h-32 text-center text-muted-foreground">
                        {deleted ? t("trash.empty") : t("list.empty")}
                      </TableCell>
                    </TableRow>
                  ) : null}
                </TableBody>
              </Table>
              <PageControls page={page} onPageChange={(number) => setParameters({ page: String(number), selected: null })} />
            </div>
          )}
        </PageContent>
      </section>

      <Sheet open={Boolean(selected)} onOpenChange={(open) => !open && closeDetail()}>
        <SheetContent
          className="w-full gap-0 p-0 sm:max-w-xl"
          onOpenAutoFocus={(event) => {
            event.preventDefault()
            detailTitleRef.current?.focus()
          }}
        >
          <SheetHeader className="border-b px-6 py-4 pr-12">
            <SheetTitle ref={detailTitleRef} tabIndex={-1} className="outline-none">
              {scope === "internal"
                ? detailUser?.displayName ?? t("detail.memberTitle")
                : detail?.contact.displayName || t("anonymous")}
            </SheetTitle>
            <SheetDescription>
              {scope === "internal" ? t("detail.memberDescription") : t("detail.contactDescription")}
            </SheetDescription>
          </SheetHeader>
          <ScrollArea className="min-h-0 flex-1">
            <div className="p-6">
              {detailLoading ? (
                <div className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground">
                  <LoaderCircleIcon className="size-4 animate-spin" />
                  {t("loading")}
                </div>
              ) : scope === "internal" && detailUser ? (
                <dl className="grid gap-5 text-sm">
                  <MemberDetailItem
                    label={t("columns.name")}
                    value={detailUser.displayName}
                    emphasized
                  />
                  <MemberDetailItem
                    label={t("columns.email")}
                    value={detailUser.email}
                  />
                  <MemberDetailItem
                    label={t("columns.role")}
                    value={userRoleLabel(detailUser.role, t)}
                  />
                  <MemberDetailItem
                    label={t("columns.status")}
                    value={
                      <UserStatusBadge
                        status={detailUser.status}
                        label={userStatusLabel(detailUser.status, t)}
                      />
                    }
                  />
                  <MemberDetailItem
                    label={t("columns.createdAt")}
                    value={formatDateTime(detailUser.createdAt)}
                  />
                </dl>
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

      <Dialog open={creating} onOpenChange={(open) => !open && setParameters({ new: null })}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("detail.createTitle")}</DialogTitle>
            <DialogDescription>{t("detail.createDescription")}</DialogDescription>
          </DialogHeader>
          <ContactForm
            channels={channels}
            onSaved={() => {
              setParameters({ new: null })
              setRefreshVersion((current) => current + 1)
            }}
            onCancel={() => setParameters({ new: null })}
          />
        </DialogContent>
      </Dialog>

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
            <AlertDialogDescription>{t("delete.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("delete.cancel")}</AlertDialogCancel>
            <AlertDialogAction disabled={deleting} onClick={() => void removeContact()}>
              {deleting ? t("delete.deleting") : t("delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageSplit>
  )
}
