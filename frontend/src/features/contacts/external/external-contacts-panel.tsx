/** 外部联系人列表、筛选、详情和回收站面板。 */
import { useEffect, useState } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ContactMethodType,
  ContactSort,
  ContactStage,
  deleteContact,
  getContact,
  isApiError,
  listContacts,
  listDeletedContacts,
  restoreContact,
  sessionPath,
  type ChannelOption,
  type ContactSummary,
  type RoleData,
  type Team,
} from "@/api"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { PageHeader } from "@/components/page-header"
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactDetailSheet } from "@/features/contacts/contact-detail-sheet"
import { ContactListLayout } from "@/features/contacts/contact-list-layout"
import { ContactScopeMobileSelect } from "@/features/contacts/contact-scope-mobile-select"
import { ContactDetailView } from "@/features/contacts/external/contact-detail"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { optionalWailsEnum } from "@/lib/wails-enum"

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

/** 外部联系人范围的列表、详情和弹窗。 */
export function ExternalContactsPanel({
  channels,
  roles,
  teams,
}: {
  channels: ChannelOption[]
  roles: RoleData[]
  teams: Team[]
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const invalidate = useResourceInvalidator()
  const {
    searchParams,
    setParameters,
    query,
    search,
    setSearch,
    currentPage,
    selected,
  } = useContactSearch()
  const deleted = searchParams.get("view") === "trash"
  const channelId = searchParams.get("channelId") ?? ""
  const stage = optionalWailsEnum(ContactStage, searchParams.get("stage"))
  const methodType = optionalWailsEnum(
    ContactMethodType,
    searchParams.get("methodType"),
  )
  const sort =
    optionalWailsEnum(ContactSort, searchParams.get("sort")) ??
    ContactSort.ContactSortCreatedAtDescending
  const [deletingContact, setDeletingContact] = useState<ContactSummary | null>(
    null,
  )
  const [restoringContact, setRestoringContact] =
    useState<ContactSummary | null>(null)
  const [deleting, setDeleting] = useState(false)

  const listParameters = {
    query,
    stage,
    channelId: deleted ? "" : channelId,
    methodType,
    sort,
    page: currentPage,
    pageSize: 50,
  }
  const list = useResource(resourceKeys.contacts({ deleted, ...listParameters }), () =>
    (deleted ? listDeletedContacts : listContacts)(listParameters),
  )
  const contacts = list.data?.contacts ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  const detail = useResource(
    resourceKeys.contact(selected),
    () => getContact(selected),
    { enabled: Boolean(selected) },
  )
  const detailContact = selected ? (detail.data ?? null) : null

  const detailError = detail.error
  useEffect(() => {
    if (!selected || !detailError) return
    if (isApiError(detailError) && sessionPath(detailError.state)) return
    console.warn("联系人详情加载失败", detailError)
    toast.error(t("detail.loadError"))
    setParameters({ selected: null })
  }, [detailError, selected, setParameters, t])

  /** 关闭联系人详情。 */
  function closeDetail() {
    setParameters({ selected: null, new: null })
  }

  /** 刷新列表并关闭详情。 */
  function refreshAndClose() {
    closeDetail()
    void invalidate(resourceKeys.contacts())
  }

  /** 将联系人移入回收站。 */
  async function removeContact() {
    if (!deletingContact) {
      return
    }
    setDeleting(true)
    try {
      await deleteContact(deletingContact.id)
      void invalidate(resourceKeys.contact(deletingContact.id))
      console.info("联系人已移入回收站", { contact_id: deletingContact.id })
      toast.success(t("delete.success"))
      setDeletingContact(null)
      if (selected === deletingContact.id) {
        closeDetail()
      }
      void invalidate(resourceKeys.contacts())
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
      void invalidate(resourceKeys.contact(restoringContact.id))
      console.info("联系人已恢复", { contact_id: restoringContact.id })
      toast.success(t("trash.restored"))
      setRestoringContact(null)
      void invalidate(resourceKeys.contacts())
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

  const hasExternalFilters = Boolean(stage || methodType)
  const selectedChannel = channels.find((channel) => channel.id === channelId)

  return (
    <>
      <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <PageHeader
          title={selectedChannel?.name ?? t("scopes.external")}
          beforeTitle={
            <ContactScopeMobileSelect
              scope="external"
              channelId={channelId}
              teams={teams}
              channels={channels}
            />
          }
        />

        <ListToolbar>
          <ListToolbarSearch
            value={search}
            aria-label={t("search.external")}
            placeholder={t("search.external")}
            onChange={(event) => setSearch(event.target.value)}
          />
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
          {!deleted ? (
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
        </ListToolbar>

        <ContactListLayout
          loading={list.loading}
          error={Boolean(list.error)}
          onRetry={() => void list.refresh()}
          page={page}
          onPageChange={(number) =>
            setParameters({ page: String(number), selected: null })
          }
        >
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>{t("columns.name")}</TableHead>
                <TableHead>{t("columns.stage")}</TableHead>
                <TableHead>{t("columns.email")}</TableHead>
                <TableHead>{t("columns.phone")}</TableHead>
                <TableHead>{t("columns.channels")}</TableHead>
                <TableHead>
                  {deleted ? t("columns.deletedAt") : t("columns.addedAt")}
                </TableHead>
                <TableHead className="text-right">
                  {t("columns.actions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {contacts.map((contact) => (
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
              ))}
              {contacts.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell
                    colSpan={7}
                    className="h-32 text-center text-muted-foreground"
                  >
                    {deleted ? t("trash.empty") : t("list.empty")}
                  </TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>
        </ContactListLayout>
      </section>

      <ContactDetailSheet
        open={Boolean(selected)}
        onClose={closeDetail}
        title={detailContact?.contact.displayName || t("anonymous")}
        description={t("detail.contactDescription")}
        loading={detail.loading && Boolean(selected)}
      >
        {detailContact ? (
          <ContactDetailView
            detail={detailContact}
            onSaved={(saved) => {
              void invalidate(resourceKeys.contact(saved.contact.id))
              void invalidate(resourceKeys.contacts())
            }}
            onNotFound={refreshAndClose}
          />
        ) : null}
      </ContactDetailSheet>

      <ContactCreateDialogs
        scope="external"
        channels={channels}
        roles={roles}
        teams={teams}
        searchParams={searchParams}
        setParameters={setParameters}
      />

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
    </>
  )
}
