/** 外部联系人列表、筛选、详情和回收站面板。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  ChannelType,
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
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { ContactCreateDialogs } from "@/features/contacts/contact-create-dialogs"
import { ContactDetailSheet } from "@/features/contacts/contact-detail-sheet"
import { ContactListSection } from "@/features/contacts/contact-list-section"
import { ContactDetailView } from "@/features/contacts/external/contact-detail"
import { channelTypeLabel } from "@/features/contacts/external/contact-labels"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { optionalWailsEnum } from "@/lib/wails-enum"

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
  const { t } = useTranslation(["contacts", "common"])
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
  // 中间栏选中渠道类别时，列出该类别下所有渠道的联系人。
  const channelType = optionalWailsEnum(ChannelType, searchParams.get("channelType"))
  const stage = optionalWailsEnum(ContactStage, searchParams.get("stage"))
  const methodType = optionalWailsEnum(
    ContactMethodType,
    searchParams.get("methodType"),
  )
  const sort =
    optionalWailsEnum(ContactSort, searchParams.get("sort")) ??
    ContactSort.ContactSortCreatedAtDescending

  const listParameters = {
    query,
    stage,
    channelId: deleted ? "" : channelId,
    channelType: deleted ? undefined : channelType,
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

  const deletion = useConfirmedAction<ContactSummary>({
    action: (contact) => deleteContact(contact.id),
    invalidateKeys: (contact) => [resourceKeys.contact(contact.id), resourceKeys.contacts()],
    successMessage: () => t("delete.success"),
    errorMessage: () => t("delete.error"),
    logLabel: "删除联系人",
    // 删除正在查看的联系人时关闭详情。
    onSuccess: (contact) => {
      if (selected === contact.id) closeDetail()
    },
  })

  const restoration = useConfirmedAction<ContactSummary>({
    action: (contact) => restoreContact(contact.id),
    invalidateKeys: (contact) => [resourceKeys.contact(contact.id), resourceKeys.contacts()],
    successMessage: () => t("trash.restored"),
    errorMessage: () => t("trash.restoreError"),
    logLabel: "恢复联系人",
  })

  const hasExternalFilters = Boolean(stage || methodType)
  const selectedChannel = channels.find((channel) => channel.id === channelId)

  return (
    <>
      <ContactListSection
        title={
          selectedChannel?.name ??
          (channelType ? channelTypeLabel(channelType, t) : t("scopes.external"))
        }
        description={t("scopeDescriptions.external")}
        scope={{ scope: "external", channelId, teams, channels }}
        toolbar={
          <>
            <ListToolbarSearch
              value={search}
              aria-label={t("search.external")}
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
                    {t("common:actions.clearFilters")}
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
          </>
        }
        list={list}
        page={page}
        setParameters={setParameters}
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "name",
              header: t("columns.name"),
              cell: (contact) => (
                <ResourceRowIdentity
                  avatar={{
                    imageURL: contact.avatarUrl,
                    name: contact.displayName || t("anonymous"),
                  }}
                  name={contact.displayName || t("anonymous")}
                  secondary={contact.stage ? t(`stages.${contact.stage}`) : null}
                  // 第二行只展示已填写的邮箱和电话。
                  description={[contact.primaryEmail, contact.primaryPhone]
                    .filter(Boolean)
                    .join(" · ")}
                />
              ),
            },
            {
              key: "channels",
              header: t("columns.channels"),
              cellClassName: "max-w-40 truncate text-muted-foreground",
              cell: (contact) =>
                t("list.source", { channel: contact.sourceChannelName }),
            },
            {
              key: "time",
              header: deleted ? t("columns.deletedAt") : t("columns.addedAt"),
              cellClassName: "whitespace-nowrap text-muted-foreground",
              cell: (contact) =>
                deleted && contact.deletedAt
                  ? t("trash.deletedAt", {
                      time: formatDateTime(contact.deletedAt),
                    })
                  : t("list.addedAt", {
                      time: formatDateTime(contact.createdAt),
                    }),
            },
          ]}
          rows={contacts}
          rowKey={(contact) => contact.id}
          empty={deleted ? t("trash.empty") : t("list.empty")}
          onRowActivate={
            deleted ? undefined : (contact) => setParameters({ selected: contact.id })
          }
          rowActions={(contact) => [
            deleted
              ? {
                  key: "restore",
                  label: t("trash.restore"),
                  onSelect: () => restoration.select(contact),
                }
              : {
                  key: "delete",
                  label: t("common:actions.delete"),
                  destructive: true,
                  separatorBefore: true,
                  onSelect: () => deletion.select(contact),
                },
          ]}
        />
      </ContactListSection>

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

      <ConfirmationDialog
        {...deletion.dialog}
        title={t("delete.title", {
          name: deletion.item?.displayName || t("anonymous"),
        })}
        description={t("delete.description")}
        pendingLabel={t("common:actions.deleting")}
      />

      <ConfirmationDialog
        {...restoration.dialog}
        title={t("trash.restoreTitle", {
          name: restoration.item?.displayName || t("anonymous"),
        })}
        description={t("trash.restoreDescription")}
        destructive={false}
        pendingLabel={t("trash.restoring")}
      />
    </>
  )
}
