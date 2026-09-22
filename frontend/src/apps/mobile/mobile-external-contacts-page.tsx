/** 移动端外部联系人列表与只读详情。 */
import { ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useParams } from "react-router"

import {
  ContactMethodType,
  ContactStage,
  getContact,
  isNotFoundApiError,
  listContacts,
} from "@/api"
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
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { useResource } from "@/hooks/use-resource"
import { useListSearchParams } from "@/hooks/use-list-search-params"

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

/** 防抖同步搜索条件，展示现有外部联系人。 */
export function MobileExternalContactsPage() {
  const { t } = useTranslation(["contacts", "mobile"])
  const { listPageCounts, scrollPositions } = useMobileNavigation()
  // 检索词变化时重置目标查询的加载进度和滚动位置。
  const { query: queryText, search, setSearch } = useListSearchParams({
    onQueryChange: (query) => {
      const storageKey = `external:${query}`
      listPageCounts.delete(storageKey)
      scrollPositions.delete(storageKey)
    },
  })

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t("mobile:contacts.external")}
        backTo="/contacts"
      />
      <MobileSearchBar
        label={t("search.external")}
        value={search}
        onChange={setSearch}
      />
      <MobileExternalContactList
        key={`external:${queryText.trim()}`}
        queryText={queryText.trim()}
        searching={search !== queryText}
      />
    </section>
  )
}

/** 逐页读取联系人，行内展示阶段、主要联系方式和来源渠道。 */
function MobileExternalContactList({
  queryText,
  searching,
}: {
  queryText: string
  searching: boolean
}) {
  const { t } = useTranslation(["contacts", "mobile"])
  return (
    <MobilePagedList
      storageKey={`external:${queryText}`}
      searching={searching}
      labels={{
        loadError: t("mobile:external.loadError"),
        loadMoreError: t("mobile:contacts.loadMoreError"),
        empty: t("mobile:external.empty"),
        allLoaded: t("mobile:external.allLoaded"),
      }}
      source={(page) => {
        const query = { query: queryText, page, pageSize: 50 }
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

/** 展示联系人的基本信息、联系方式、备注和关联渠道。 */
export function MobileExternalContactPage() {
  const { t } = useTranslation(["contacts", "mobile", "common"])
  const { contactID = "" } = useParams()
  const { formatDateTime } = useDateTime()
  const {
    data: detail,
    loading,
    error,
    refresh,
  } = useResource(
    resourceKeys.contact(contactID),
    () => getContact(contactID),
    {
      staleTime: 0,
    },
  )
  const empty = t("detail.empty")
  const stageKey = detail ? contactStageKey(detail.contact.stage) : null
  const primaryEmail = detail?.methods.find(
    (method) => method.type === ContactMethodType.ContactMethodTypeEmail,
  )
  const primaryPhone = detail?.methods.find(
    (method) => method.type === ContactMethodType.ContactMethodTypePhone,
  )

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
        {detail ? (
          <div>
            <div className="flex items-center gap-3 pb-6">
              <ProfileAvatar
                name={detail.contact.displayName}
                imageURL={detail.avatarUrl}
                className="size-14 text-xl"
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
            <dl className="divide-y border-y">
              <div className="py-4">
                <dt className="text-xs text-muted-foreground">
                  {t("form.email")}
                </dt>
                <dd className="mt-1 break-all text-sm">
                  {primaryEmail?.value || empty}
                </dd>
              </div>
              <div className="py-4">
                <dt className="text-xs text-muted-foreground">
                  {t("form.phone")}
                </dt>
                <dd className="mt-1 break-all text-sm">
                  {primaryPhone?.value || empty}
                </dd>
              </div>
              <div className="py-4">
                <dt className="text-xs text-muted-foreground">
                  {t("detail.sourceChannel")}
                </dt>
                <dd className="mt-1 break-words text-sm">
                  {detail.sourceChannel.name}
                </dd>
              </div>
              <div className="py-4">
                <dt className="text-xs text-muted-foreground">
                  {t("form.notes")}
                </dt>
                <dd className="mt-1 break-words whitespace-pre-wrap text-sm">
                  {detail.contact.notes || empty}
                </dd>
              </div>
              <div className="py-4">
                <dt className="text-xs text-muted-foreground">
                  {t("columns.addedAt")}
                </dt>
                <dd className="mt-1 text-sm">
                  {formatDateTime(detail.contact.createdAt)}
                </dd>
              </div>
              <div className="py-4">
                <dt className="text-xs text-muted-foreground">
                  {t("detail.linkedChannels")}
                </dt>
                <dd className="mt-1 space-y-2 text-sm">
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
                </dd>
              </div>
            </dl>
          </div>
        ) : null}
      </MobileScrollArea>
    </section>
  )
}
