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

import { listChannels, type ChannelSummary } from "@/api/channels"
import {
  getContact,
  deleteContact,
  listContacts,
  listDeletedContacts,
  restoreContact,
  type ContactDetail,
  type ContactListResponse,
  type ContactMethodType,
  type ContactStage,
  type ContactSummary,
} from "@/api/contacts"
import { ApiError } from "@/api/client"
import {
  getUser,
  listUsers,
  type DirectoryUser,
} from "@/api/users"
import type { PageInfo } from "@/api/types"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
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
import { useDateTime } from "@/hooks/use-date-time"
import { cn } from "@/lib/utils"

export type ContactScope = "internal" | "external" | "agents"

type LoadState = "loading" | "ready" | "error"

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
        "flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
        active && "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
      )}
      onClick={onClick}
    >
      <Icon className="size-4 shrink-0" />
      <span className="truncate">{children}</span>
    </button>
  )
}

function SubscopeButton({
  active,
  children,
  onClick,
  icon: Icon,
}: {
  active: boolean
  children: React.ReactNode
  onClick: () => void
  icon?: typeof GlobeIcon
}) {
  return (
    <button
      type="button"
      className={cn(
        "flex min-h-8 w-full items-center gap-2 rounded-md py-1.5 pr-2 pl-8 text-left text-sm text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
        active && "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
      )}
      onClick={onClick}
    >
      {Icon ? <Icon className="size-3.5 shrink-0" /> : null}
      <span className="truncate">{children}</span>
    </button>
  )
}

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
    const groups = new Map<string, ChannelSummary[]>()
    for (const channel of channels) {
      groups.set(channel.type, [...(groups.get(channel.type) ?? []), channel])
    }
    return [...groups.entries()]
  }, [channels])

  return (
    <aside className="hidden w-60 shrink-0 border-r bg-sidebar text-sidebar-foreground md:flex md:flex-col">
      <ScrollArea className="min-h-0 flex-1">
        <nav className="flex flex-col gap-1 p-3" aria-label={t("scopeNavigation")}>
          <ScopeButton
            active={scope === "internal"}
            icon={UsersIcon}
            onClick={() => navigate("/contacts/internal")}
          >
            {t("scopes.internal")}
          </ScopeButton>

          <Collapsible defaultOpen>
            <CollapsibleTrigger asChild>
              <button
                type="button"
                className={cn(
                  "group flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                  scope === "external" && "font-medium text-sidebar-accent-foreground",
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
                      className="group flex min-h-8 w-full items-center gap-2 rounded-md py-1.5 pr-2 pl-8 text-left text-sm text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                    >
                      <ChevronRightIcon className="size-3.5 transition-transform group-data-[state=open]:rotate-90" />
                      <span>{t(`channelTypes.${type}`, { defaultValue: type })}</span>
                    </button>
                  </CollapsibleTrigger>
                  <CollapsibleContent className="flex flex-col gap-0.5">
                    {items.map((channel) => (
                      <SubscopeButton
                        key={channel.id}
                        active={scope === "external" && channelId === channel.id}
                        icon={type === "website" ? GlobeIcon : undefined}
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

          <ScopeButton
            active={scope === "agents"}
            icon={BotIcon}
            onClick={() => navigate("/contacts/agents")}
          >
            {t("scopes.agents")}
          </ScopeButton>
        </nav>
      </ScrollArea>
    </aside>
  )
}

function StageLabel({ stage }: { stage: ContactStage }) {
  const { t } = useTranslation("contacts")
  return (
    <span className="inline-flex rounded-md bg-secondary px-2 py-0.5 text-xs font-medium text-secondary-foreground">
      {t(`stages.${stage}`)}
    </span>
  )
}

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
  const stage = (searchParams.get("stage") ?? "") as ContactStage | ""
  const methodType = (searchParams.get("methodType") ?? "") as ContactMethodType | ""
  const sort = (searchParams.get("sort") ?? "createdAt.desc") as
    | "updatedAt.desc"
    | "createdAt.desc"
    | "displayName.asc"
  const status = searchParams.get("status") ?? ""
  const role = searchParams.get("role") ?? ""
  const currentPage = Number(searchParams.get("page") ?? "1") || 1

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
        if (!(error instanceof DOMException && error.name === "AbortError")) {
          setChannels([])
        }
      })
    return () => controller.abort()
  }, [])

  const loadList = useCallback(
    async (signal?: AbortSignal) => {
      setLoadState("loading")
      try {
        if (scope === "external") {
          const loader = deleted ? listDeletedContacts : listContacts
          const response: ContactListResponse = await loader(
            {
              q: query,
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
            { q: query, status, role, page: currentPage, pageSize: 50 },
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
        if (error instanceof ApiError && error.code === "AUTH_REQUIRED") {
          navigate("/login", { replace: true })
          return
        }
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
        if (error instanceof ApiError && error.code === "AUTH_REQUIRED") {
          navigate("/login", { replace: true })
          return
        }
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

  function closeDetail() {
    setParameters({ selected: null, new: null })
    setDetail(null)
    setDetailUser(null)
  }

  function refreshAndClose() {
    closeDetail()
    setRefreshVersion((current) => current + 1)
  }

  async function removeContact() {
    if (!deletingContact) {
      return
    }
    setDeleting(true)
    try {
      await deleteContact(deletingContact.id)
      toast.success(t("delete.success"))
      setDeletingContact(null)
      if (selected === deletingContact.id) {
        closeDetail()
      }
      setRefreshVersion((current) => current + 1)
    } catch (error) {
      if (error instanceof ApiError && error.code === "AUTH_REQUIRED") {
        navigate("/login", { replace: true })
        return
      }
      toast.error(t("delete.error"))
    } finally {
      setDeleting(false)
    }
  }

  async function restore(item: ContactSummary) {
    try {
      await restoreContact(item.id)
      toast.success(t("trash.restored"))
      setRefreshVersion((current) => current + 1)
    } catch (error) {
      if (error instanceof ApiError && error.code === "AUTH_REQUIRED") {
        navigate("/login", { replace: true })
        return
      }
      toast.error(t("trash.restoreError"))
    }
  }

  const hasExternalFilters = Boolean(stage || methodType)
  const hasInternalFilters = Boolean(status || role)

  return (
    <div className="flex min-h-0 w-full flex-1 overflow-hidden">
      <ContactScopeSidebar
        scope={scope}
        deleted={deleted}
        channelId={channelId}
        channels={channels}
      />

      <section className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <div className="flex flex-wrap items-center gap-3 border-b px-4 py-3 sm:px-6">
          <div className="w-full md:hidden">
            <NativeSelect
              className="h-8 w-full"
              aria-label={t("scopeNavigation")}
              value={mobileScope}
              onChange={(event) => changeMobileScope(event.target.value)}
            >
              <option value="internal">{t("scopes.internal")}</option>
              <option value="external">{t("scopes.external")} · {t("all")}</option>
              {channels.map((channel) => (
                <option key={channel.id} value={`channel:${channel.id}`}>
                  {t("scopes.external")} · {channel.name}
                </option>
              ))}
              <option value="trash">{t("trash.title")}</option>
              <option value="agents">{t("scopes.agents")}</option>
            </NativeSelect>
          </div>
          <div className="mr-auto min-w-40">
            <h2 className="font-semibold tracking-tight">{title}</h2>
            <p className="text-xs text-muted-foreground">
              {t("list.count", { count: page.total })}
            </p>
          </div>
          {scope === "external" ? (
            <>
              {deleted ? (
                <Button variant="outline" size="sm" onClick={() => navigate("/contacts/external")}>
                  {t("trash.back")}
                </Button>
              ) : (
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
              )}
            </>
          ) : null}
        </div>

        {scope !== "agents" ? (
          <ListToolbar>
            <ListToolbarSearch
              value={search}
              placeholder={scope === "internal" ? t("search.internal") : t("search.external")}
              onChange={(event) => setSearch(event.target.value)}
            />
            {scope === "internal" ? (
              <>
                <ListToolbarFilter
                  label={t("filters.status")}
                  allLabel={t("filters.allStatuses")}
                  value={status}
                  options={[
                    { value: "active", label: t("statuses.active") },
                    { value: "inactive", label: t("statuses.inactive") },
                  ]}
                  onValueChange={(value) =>
                    setParameters({ status: value || null, page: null, selected: null })
                  }
                />
                <ListToolbarFilter
                  label={t("filters.role")}
                  allLabel={t("filters.allRoles")}
                  value={role}
                  options={[
                    { value: "owner", label: t("roles.owner") },
                    { value: "member", label: t("roles.member") },
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
                  value={stage}
                  options={[
                    { value: "visitor", label: t("stages.visitor") },
                    { value: "lead", label: t("stages.lead") },
                    { value: "customer", label: t("stages.customer") },
                  ]}
                  onValueChange={(value) =>
                    setParameters({ stage: value || null, page: null, selected: null })
                  }
                />
                <ListToolbarFilter
                  label={t("filters.method")}
                  allLabel={t("filters.allMethods")}
                  value={methodType}
                  options={[
                    { value: "email", label: t("methods.email") },
                    { value: "phone", label: t("methods.phone") },
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
                    { value: "createdAt.desc", label: t("sort.created") },
                    { value: "updatedAt.desc", label: t("sort.updated") },
                    { value: "displayName.asc", label: t("sort.name") },
                  ]}
                  onValueChange={(value) =>
                    setParameters({ sort: value, page: null, selected: null })
                  }
                />
              </div>
            ) : null}
          </ListToolbar>
        ) : null}

        <div className="min-h-0 flex-1 overflow-auto p-4 sm:p-6">
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
                        <TableRow
                          key={user.id}
                          tabIndex={0}
                          className="cursor-pointer"
                          onClick={() => setParameters({ selected: user.id })}
                          onKeyDown={(event) => {
                            if (event.key === "Enter" || event.key === " ") {
                              setParameters({ selected: user.id })
                            }
                          }}
                        >
                          <TableCell className="font-medium">{user.displayName}</TableCell>
                          <TableCell className="text-muted-foreground">{user.email}</TableCell>
                          <TableCell>{t(`roles.${user.role}`, { defaultValue: user.role })}</TableCell>
                          <TableCell>{t(`statuses.${user.status}`, { defaultValue: user.status })}</TableCell>
                        </TableRow>
                      ))
                    : null}
                  {scope === "external" && contacts.length > 0
                    ? contacts.map((contact) => (
                        <TableRow
                          key={contact.id}
                          tabIndex={deleted ? undefined : 0}
                          className={cn(!deleted && "cursor-pointer")}
                          onClick={() => !deleted && setParameters({ selected: contact.id })}
                          onKeyDown={(event) => {
                            if (!deleted && event.target === event.currentTarget && (event.key === "Enter" || event.key === " ")) {
                              setParameters({ selected: contact.id })
                            }
                          }}
                        >
                          <TableCell className="font-medium">{contact.displayName || t("anonymous")}</TableCell>
                          <TableCell><StageLabel stage={contact.stage} /></TableCell>
                          <TableCell className="text-muted-foreground">{contact.primaryEmail || "—"}</TableCell>
                          <TableCell className="text-muted-foreground">{contact.primaryPhone || "—"}</TableCell>
                          <TableCell>{contact.sourceChannelName}</TableCell>
                          <TableCell className="whitespace-nowrap text-muted-foreground">
                            {formatDateTime(deleted && contact.deletedAt ? contact.deletedAt : contact.createdAt)}
                          </TableCell>
                          {deleted ? (
                            <TableCell className="text-right">
                              <Button variant="ghost" size="sm" onClick={() => void restore(contact)}>
                                {t("trash.restore")}
                              </Button>
                            </TableCell>
                          ) : (
                            <TableCell className="text-right" onClick={(event) => event.stopPropagation()}>
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
                            </TableCell>
                          )}
                        </TableRow>
                      ))
                    : null}
                  {((scope === "internal" && users.length === 0) || (scope === "external" && contacts.length === 0)) ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={scope === "internal" ? 4 : 7} className="h-32 text-center text-muted-foreground">
                        {deleted ? t("trash.empty") : t("list.empty")}
                      </TableCell>
                    </TableRow>
                  ) : null}
                </TableBody>
              </Table>
              <PageControls page={page} onPageChange={(number) => setParameters({ page: String(number), selected: null })} />
            </div>
          )}
        </div>
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
                  <div><dt className="text-muted-foreground">{t("columns.name")}</dt><dd className="mt-1 font-medium">{detailUser.displayName}</dd></div>
                  <div><dt className="text-muted-foreground">{t("columns.email")}</dt><dd className="mt-1">{detailUser.email}</dd></div>
                  <div><dt className="text-muted-foreground">{t("columns.role")}</dt><dd className="mt-1">{t(`roles.${detailUser.role}`, { defaultValue: detailUser.role })}</dd></div>
                  <div><dt className="text-muted-foreground">{t("columns.status")}</dt><dd className="mt-1">{t(`statuses.${detailUser.status}`, { defaultValue: detailUser.status })}</dd></div>
                  <div><dt className="text-muted-foreground">{t("columns.createdAt")}</dt><dd className="mt-1">{formatDateTime(detailUser.createdAt)}</dd></div>
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
    </div>
  )
}
