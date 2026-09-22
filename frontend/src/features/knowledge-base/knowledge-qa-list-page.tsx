/** 本地知识问答的分组列表、搜索和删除操作。 */
import { useEffect, useRef, useState } from "react"
import { PlusIcon, SearchCheckIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useParams } from "react-router"

import {
  KnowledgeBaseCategory,
  deleteKnowledgeQAEntry,
  getKnowledgeBase,
  listKnowledgeQAEntries,
  type KnowledgeQASummaryData,
} from "@/api"
import {
  ListToolbar,
  ListToolbarSearch,
  ListToolbarReset,
} from "@/components/list-toolbar"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useListSearchParams } from "@/hooks/use-list-search-params"
import { useListScrollRestore } from "@/hooks/use-list-scroll-restore"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { KnowledgeQATable } from "@/features/knowledge-base/knowledge-qa-table"
import { ResourceContent } from "@/components/resource-content"
import { KnowledgeRetrievalSheet } from "@/features/knowledge-base/knowledge-retrieval-sheet"

/** 按分组切换列表实例，隔离删除对话框和滚动恢复状态。 */
export function KnowledgeQAListPage() {
  const { knowledgeBaseId = "", groupId = "" } = useParams()
  return (
    <KnowledgeQAGroupList
      key={`${knowledgeBaseId}/${groupId}`}
      knowledgeBaseId={knowledgeBaseId}
      groupId={groupId}
    />
  )
}

/** 展示当前分组中的问答，并保留返回时的列表位置。 */
function KnowledgeQAGroupList({
  knowledgeBaseId,
  groupId,
}: {
  knowledgeBaseId: string
  groupId: string
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const location = useLocation()
  const { searchParams, query, search, setSearch, setParameters } =
    useListSearchParams()
  const parsedPage = Number(searchParams.get("page") ?? 1)
  const pageNumber =
    Number.isSafeInteger(parsedPage) && parsedPage > 0 ? parsedPage : 1
  const parameters = { groupId, keyword: query, page: pageNumber, pageSize: 20 }
  const base = useResource(
    resourceKeys.knowledgeBase(knowledgeBaseId),
    (signal) => getKnowledgeBase(knowledgeBaseId, signal),
  )
  const list = useResource(
    resourceKeys.knowledgeQAEntries(knowledgeBaseId, parameters),
    (signal) => listKnowledgeQAEntries(knowledgeBaseId, parameters, signal),
    {
      staleTime: 0,
      keepPreviousData: true,
      refetchInterval: (data) =>
        data?.entries.some(
          (entry) => entry.status === "queued" || entry.status === "running",
        )
          ? 2000
          : false,
    },
  )
  const groups =
    base.data?.groups.flatMap((group) => [group, ...group.children]) ?? []
  const group = groups.find((item) => item.id === groupId)
  const groupName = group?.isDefault ? t("group.default") : group?.name
  const listPath = `/knowledge-bases/${knowledgeBaseId}/groups/${groupId}/qa`
  const scroll = useListScrollRestore(
    `${listPath}${location.search}`,
    Boolean(list.data && !list.isPlaceholderData && base.data),
  )
  const deletion = useConfirmedAction<KnowledgeQASummaryData>({
    action: (entry) => deleteKnowledgeQAEntry(knowledgeBaseId, entry.id),
    invalidateKeys: (entry) => [
      resourceKeys.knowledgeQAEntries(knowledgeBaseId),
      resourceKeys.knowledgeQAEntry(knowledgeBaseId, entry.id),
    ],
    logLabel: "问答删除",
    successMessage: () => t("qa.deleteSuccess"),
    errorMessage: () => t("qa.deleteError"),
  })
  const [retrievalOpen, setRetrievalOpen] = useState(false)
  const retrievalTrigger = useRef<HTMLButtonElement>(null)
  const totalPages = Math.max(1, Math.ceil((list.data?.page.total ?? 0) / 20))

  useEffect(() => {
    if (list.data && !list.isPlaceholderData && pageNumber > totalPages)
      setParameters(
        { page: totalPages === 1 ? null : String(totalPages) },
        true,
      )
  }, [list.data, list.isPlaceholderData, pageNumber, setParameters, totalPages])

  return (
    <>
      <PageHeader
        title={
          [base.data?.name, groupName].filter(Boolean).join(" · ") ||
          t("qa.title")
        }
        description={t("qa.description")}
      >
        {list.data && !list.error ? (
          <>
            <Button
              ref={retrievalTrigger}
              variant="ghost"
              size="icon-sm"
              className="shrink-0"
              aria-label={t("retrieval.action")}
              title={t("retrieval.action")}
              onClick={() => setRetrievalOpen(true)}
            >
              <SearchCheckIcon />
            </Button>
            <Button variant="ghost" size="icon-sm" asChild>
              <Link
                to={`${listPath}/new${location.search}`}
                aria-label={t("qa.create")}
                title={t("qa.create")}
              >
                <PlusIcon />
              </Link>
            </Button>
          </>
        ) : null}
      </PageHeader>
      <ListToolbar>
        <ListToolbarSearch
          value={search}
          aria-label={t("qa.search")}
          onChange={(event) => setSearch(event.target.value)}
        />
        {search ? (
          <ListToolbarReset
            onClick={() => {
              setSearch("")
              setParameters({ q: null, page: null })
            }}
          >
            {t("common:actions.clearFilters")}
          </ListToolbarReset>
        ) : null}
      </ListToolbar>
      <PageContent ref={scroll.ref} onScroll={scroll.onScroll}>
        <ResourceContent resources={[base, list]} errorMessage={t("qa.loadError")}>
          <KnowledgeQATable
            knowledgeBaseId={knowledgeBaseId}
            data={list.data!}
            loading={list.isPlaceholderData || list.refreshing}
            listPath={listPath}
            search={location.search}
            filtered={query !== ""}
            onDelete={deletion.select}
            onPageChange={(page) =>
              setParameters({ page: page === 1 ? null : String(page) })
            }
          />
        </ResourceContent>
      </PageContent>
      <ConfirmationDialog
        {...deletion.dialog}
        title={t("qa.deleteTitle")}
        // 问题原文可能含换行或长串字符，保留换行并允许断词。
        description={
          <span className="whitespace-pre-wrap break-words">
            {t("qa.deleteDescription", {
              question: deletion.item?.question ?? "",
            })}
          </span>
        }
        pendingLabel={t("common:actions.deleting")}
      />
      <KnowledgeRetrievalSheet
        open={retrievalOpen}
        onOpenChange={setRetrievalOpen}
        knowledgeBaseId={knowledgeBaseId}
        category={KnowledgeBaseCategory.KnowledgeBaseCategoryQA}
        triggerRef={retrievalTrigger}
      />
    </>
  )
}
