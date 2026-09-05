/** 本地知识问答的分组列表、搜索和删除操作。 */
import { useEffect, useLayoutEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useParams } from "react-router"

import {
  getKnowledgeBase,
  listKnowledgeQAEntries,
  type KnowledgeQASummaryData,
} from "@/api"
import {
  ListToolbar,
  ListToolbarSearch,
  ListToolbarReset,
} from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { useListSearchParams } from "@/hooks/use-list-search-params"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { useKnowledgeBaseContext } from "@/features/knowledge-base/knowledge-base-context"
import { KnowledgeQATable } from "@/features/knowledge-base/knowledge-qa-table"
import { KnowledgeQADeleteDialog } from "@/features/knowledge-base/knowledge-qa-delete-dialog"
import { KnowledgeQAFeedback } from "@/features/knowledge-base/knowledge-qa-feedback"

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
  const { t } = useTranslation("knowledgeBase")
  const location = useLocation()
  const { qaListScrollPositions } = useKnowledgeBaseContext()
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
    { staleTime: 0, keepPreviousData: true },
  )
  const groups =
    base.data?.groups.flatMap((group) => [group, ...group.children]) ?? []
  const group = groups.find((item) => item.id === groupId)
  const groupName = group?.isDefault ? t("group.default") : group?.name
  const listPath = `/knowledge-bases/${knowledgeBaseId}/groups/${groupId}/qa`
  const scrollKey = `${listPath}${location.search}`
  const scrollContainer = useRef<HTMLDivElement>(null)
  const restoredKey = useRef("")
  const [deleting, setDeleting] = useState<KnowledgeQASummaryData | null>(null)
  const totalPages = Math.max(1, Math.ceil((list.data?.page.total ?? 0) / 20))

  // 数据就绪后恢复对应分组和筛选条件的滚动位置。
  useLayoutEffect(() => {
    if (
      !list.data ||
      list.isPlaceholderData ||
      !base.data ||
      !scrollContainer.current ||
      restoredKey.current === scrollKey
    )
      return
    scrollContainer.current.scrollTop =
      qaListScrollPositions.get(scrollKey) ?? 0
    restoredKey.current = scrollKey
  }, [
    base.data,
    list.data,
    list.isPlaceholderData,
    qaListScrollPositions,
    scrollKey,
  ])
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
      >
        {list.data && !list.error ? (
          <Button size="sm" asChild>
            <Link to={`${listPath}/new${location.search}`}>
              {t("qa.create")}
            </Link>
          </Button>
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
            {t("documents.filters.clear")}
          </ListToolbarReset>
        ) : null}
      </ListToolbar>
      <PageContent
        ref={scrollContainer}
        onScroll={(event) => {
          if (restoredKey.current === scrollKey)
            qaListScrollPositions.set(scrollKey, event.currentTarget.scrollTop)
        }}
      >
        {base.error || list.error || !base.data || !list.data ? (
          <KnowledgeQAFeedback
            error={base.error ?? list.error}
            retry={() => void (base.error ? base.refresh() : list.refresh())}
          />
        ) : (
          <KnowledgeQATable
            data={list.data}
            loading={list.isPlaceholderData || list.refreshing}
            listPath={listPath}
            search={location.search}
            filtered={query !== ""}
            onDelete={setDeleting}
            onPageChange={(page) =>
              setParameters({ page: page === 1 ? null : String(page) })
            }
          />
        )}
      </PageContent>
      <KnowledgeQADeleteDialog
        knowledgeBaseId={knowledgeBaseId}
        entry={deleting}
        onClose={() => setDeleting(null)}
      />
    </>
  )
}
