/** 移动端外部联系人列表、阶段筛选与详情。 */
import { useState } from "react"
import { ChevronRightIcon, PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useNavigate, useParams } from "react-router"

import {
  ContactStage,
  getContact,
  isNotFoundApiError,
  listContacts,
} from "@/api"
import { editableContactFields } from "@/apps/mobile/mobile-external-contact-editor"
import { MobileFilterSheet } from "@/apps/mobile/mobile-filter-sheet"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import {
  MobilePageHeader,
  MobilePageState,
  MobileScrollArea,
  MobileSearchBar,
} from "@/apps/mobile/mobile-page"
import { MobilePagedList } from "@/apps/mobile/mobile-paged-list"
import { LoadingIndicator } from "@/components/loading-indicator"
import { ProfileAvatar } from "@/components/profile-avatar"
import { Button } from "@/components/ui/button"
import { contactValuesFromDetail } from "@/features/contacts/external/contact-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { useResource } from "@/hooks/use-resource"
import { useListSearchParams } from "@/hooks/use-list-search-params"
import { optionalWailsEnum } from "@/lib/wails-enum"

/** 联系人阶段对应的翻译键，未知阶段不展示。 */
function contactStageKey(stage: ContactStage) {
  switch (stage) {
    case ContactStage.ContactStageVisitor:
      return "stages.visitor" as const
    case ContactStage.ContactStageLead:
      return "stages.lead" as const
    case ContactStage.ContactStageCustomer:
      return "stages.customer" as const
    default:
      console.warn("未知的联系人阶段", stage)
      return null
  }
}

/** 联系人阶段筛选的可选项，空值表示全部阶段。 */
const contactStageFilters = [
  { value: "", label: "filters.allStages" },
  { value: ContactStage.ContactStageVisitor, label: "stages.visitor" },
  { value: ContactStage.ContactStageLead, label: "stages.lead" },
  { value: ContactStage.ContactStageCustomer, label: "stages.customer" },
] as const

/** 防抖同步搜索条件，按阶段筛选并展示现有外部联系人。 */
export function MobileExternalContactsPage() {
  const { t } = useTranslation(["contacts", "mobile"])
  const navigate = useNavigate()
  const location = useLocation()
  const { listPageCounts, scrollPositions } = useMobileNavigation()
  // 检索词变化时重置目标查询的加载进度和滚动位置。
  const {
    searchParams,
    setParameters,
    query: queryText,
    search,
    setSearch,
  } = useListSearchParams({
    onQueryChange: (query) => {
      const storageKey = `external:${stage ?? ""}:${query}`
      listPageCounts.delete(storageKey)
      scrollPositions.delete(storageKey)
    },
  })
  const stage = optionalWailsEnum(ContactStage, searchParams.get("stage"))
  const [draftStage, setDraftStage] = useState<ContactStage | "">("")
  const stageFilter = contactStageFilters.find(
    (item) => item.value === (stage ?? ""),
  )

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t("scopes.external")}
        backTo="/contacts"
        actions={
          <Button
            variant="ghost"
            size="icon-lg"
            className="-mr-2"
            aria-label={t("detail.createTitle")}
            title={t("detail.createTitle")}
            onClick={() =>
              navigate("/contacts/external/new", { state: { mobileBack: true } })
            }
          >
            <PlusIcon />
          </Button>
        }
      />
      <MobileSearchBar
        label={t("search.external")}
        value={search}
        onChange={setSearch}
      />
      <MobileFilterSheet
        summary={stage && stageFilter ? t(stageFilter.label) : ""}
        onOpen={() => setDraftStage(stage ?? "")}
        onReset={() => setDraftStage("")}
        onApply={() => {
          // 切换阶段时重置目标查询的加载进度和滚动位置。
          const storageKey = `external:${draftStage}:${queryText.trim()}`
          listPageCounts.delete(storageKey)
          scrollPositions.delete(storageKey)
          setParameters({ stage: draftStage || null }, true, location.state)
        }}
      >
        <div
          role="group"
          aria-label={t("filters.stage")}
          className="grid grid-cols-2 gap-2"
        >
          {contactStageFilters.map((item) => (
            <Button
              key={item.value}
              variant={draftStage === item.value ? "default" : "outline"}
              className="min-h-11"
              aria-pressed={draftStage === item.value}
              onClick={() => setDraftStage(item.value)}
            >
              {t(item.label)}
            </Button>
          ))}
        </div>
      </MobileFilterSheet>
      <MobileExternalContactList
        key={`external:${stage ?? ""}:${queryText.trim()}`}
        stage={stage}
        queryText={queryText.trim()}
        searching={search !== queryText}
      />
    </section>
  )
}

/** 逐页读取联系人，行内展示阶段、主要联系方式和来源渠道。 */
function MobileExternalContactList({
  stage,
  queryText,
  searching,
}: {
  stage: ContactStage | undefined
  queryText: string
  searching: boolean
}) {
  const { t } = useTranslation(["contacts", "mobile"])
  return (
    <MobilePagedList
      storageKey={`external:${stage ?? ""}:${queryText}`}
      searching={searching}
      labels={{
        loadError: t("mobile:external.loadError"),
        loadMoreError: t("mobile:contacts.loadMoreError"),
        empty: t("mobile:external.empty"),
        allLoaded: t("mobile:external.allLoaded"),
      }}
      source={(page) => {
        const query = { query: queryText, stage, page, pageSize: 50 }
        return {
          key: resourceKeys.contacts(query),
          load: (signal) => listContacts(query, signal),
        }
      }}
      select={(data) => ({ items: data.contacts, page: data.page })}
    >
      {(contacts) => (
        <ul className="divide-y border-b">
          {contacts.map((contact) => {
            const stageKey = contactStageKey(contact.stage)
            return (
              <li key={contact.id}>
                <Link
                  to={`/contacts/external/${contact.id}`}
                  state={{ mobileBack: true }}
                  className="flex min-h-18 items-center gap-3 px-4 py-3 outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                >
                  <ProfileAvatar
                    name={contact.displayName}
                    imageURL={contact.avatarUrl}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="truncate text-[15px] font-medium">
                        {contact.displayName || t("anonymous")}
                      </span>
                      {stageKey ? (
                        <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-xs font-medium">
                          {t(stageKey)}
                        </span>
                      ) : null}
                    </span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {[
                        contact.primaryEmail || contact.primaryPhone,
                        contact.sourceChannelName,
                      ]
                        .filter(Boolean)
                        .join(" · ")}
                    </span>
                  </span>
                  <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </MobilePagedList>
  )
}

/** 展示联系人的资料与关联渠道，资料项点进后逐项编辑。 */
export function MobileExternalContactPage() {
  const { t } = useTranslation(["contacts", "mobile", "common"])
  const { contactID = "" } = useParams()
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const {
    data: detail,
    loading,
    error,
    refresh,
  } = useResource(
    resourceKeys.contact(contactID),
    () => getContact(contactID),
    { staleTime: 0, refetchOnWindowFocus: true },
  )
  const empty = t("detail.empty")
  const values = detail ? contactValuesFromDetail(detail) : null
  const stageKey = detail ? contactStageKey(detail.contact.stage) : null

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t("mobile:external.detail")}
        backTo="/contacts/external"
        actions={
          error && detail ? (
            <Button
              variant="ghost"
              className="min-h-11"
              onClick={() => void refresh()}
            >
              {t("mobile:refreshFailed")}
            </Button>
          ) : undefined
        }
      />
      <MobileScrollArea
        storageKey={`external:${contactID}`}
        ready={Boolean(detail)}
        className="px-4 py-6"
      >
        {loading && !detail ? (
          <LoadingIndicator className="min-h-64 justify-center">
            {t("common:status.loading")}
          </LoadingIndicator>
        ) : null}
        {error && !detail ? (
          <MobilePageState
            title={t(
              isNotFoundApiError(error)
                ? "mobile:external.notFound"
                : "mobile:external.loadError",
            )}
            onRetry={() => void refresh()}
          />
        ) : null}
        {detail && values ? (
          <div>
            <div className="flex items-center gap-3 pb-6">
              <ProfileAvatar
                name={detail.contact.displayName}
                imageURL={detail.avatarUrl}
                className="size-14"
              />
              <div className="min-w-0 space-y-2">
                <h2 className="break-words text-lg font-semibold">
                  {detail.contact.displayName || t("anonymous")}
                </h2>
                {stageKey ? (
                  <span className="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium">
                    {t(stageKey)}
                  </span>
                ) : null}
              </div>
            </div>
            <div className="divide-y border-y">
              {editableContactFields.map(({ field, label }) => (
                <button
                    key={field}
                    type="button"
                    className="flex w-full items-center gap-3 py-4 text-left outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                    onClick={() =>
                      navigate(`/contacts/external/${contactID}/edit/${field}`, {
                        state: { mobileBack: true },
                      })
                    }
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block text-xs text-muted-foreground">
                        {t(label)}
                      </span>
                      <span className="block mt-1 break-words whitespace-pre-wrap text-sm">
                        {field === "stage"
                          ? stageKey
                            ? t(stageKey)
                            : empty
                          : values[field] || empty}
                      </span>
                    </span>
                  <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
                </button>
              ))}
              <div className="py-4">
                <span className="block text-xs text-muted-foreground">
                  {t("detail.sourceChannel")}
                </span>
                <span className="block mt-1 break-words text-sm">
                  {detail.sourceChannel.name}
                </span>
              </div>
              <div className="py-4">
                <span className="block text-xs text-muted-foreground">
                  {t("columns.addedAt")}
                </span>
                <span className="block mt-1 text-sm">
                  {formatDateTime(detail.contact.createdAt)}
                </span>
              </div>
              <div className="py-4">
                <span className="block text-xs text-muted-foreground">
                  {t("detail.linkedChannels")}
                </span>
                <span className="block mt-1 space-y-2 text-sm">
                  {detail.channelIdentities.length
                    ? detail.channelIdentities.map((identity) => (
                        <span
                          key={`${identity.channelId}:${identity.externalId}`}
                          className="block"
                        >
                          <span className="block break-words">
                            {identity.channelName}
                          </span>
                          <span className="block break-all text-xs text-muted-foreground">
                            {identity.displayName || identity.externalId}
                          </span>
                        </span>
                      ))
                    : empty}
                </span>
              </div>
            </div>
          </div>
        ) : null}
      </MobileScrollArea>
    </section>
  )
}
