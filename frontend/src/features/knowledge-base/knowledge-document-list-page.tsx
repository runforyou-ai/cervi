/** 标准知识库分组文档列表、上传和原地管理。 */
import { useEffect, useRef, useState } from "react"
import { SearchCheckIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useLocation, useParams } from "react-router"
import { KnowledgeBaseCategory, getKnowledgeBase, listKnowledgeDocuments } from "@/api"
import { ListToolbar, ListToolbarSearch, ListToolbarReset } from "@/components/list-toolbar"
import { PageHeader } from "@/components/page-header"
import { PageContent } from "@/components/page-content"
import { Button } from "@/components/ui/button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { useListSearchParams } from "@/hooks/use-list-search-params"
import { useListScrollRestore } from "@/hooks/use-list-scroll-restore"
import { KnowledgeQAFeedback } from "./knowledge-qa-feedback"
import { KnowledgeDocumentUpload } from "./knowledge-document-upload"
import { KnowledgeDocumentActions, type DocumentAction } from "./knowledge-document-actions"
import { KnowledgeDocumentTable } from "./knowledge-document-table"
import { KnowledgeRetrievalSheet } from "./knowledge-retrieval-sheet"

/** 按分组隔离上传批次、弹窗和滚动恢复状态。 */
export function KnowledgeDocumentListPage() {
  const { knowledgeBaseId = "", groupId = "" } = useParams()
  return <KnowledgeDocumentGroupList key={`${knowledgeBaseId}/${groupId}`} baseId={knowledgeBaseId} groupId={groupId} />
}

/** 显示创建时间倒序的文档，返回时恢复原列表位置。 */
function KnowledgeDocumentGroupList({ baseId, groupId }: { baseId: string; groupId: string }) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const location = useLocation()
  const { searchParams, query, search, setSearch, setParameters } = useListSearchParams()
  const parsed = Number(searchParams.get("page") ?? 1)
  const page = Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 1
  const parameters = { groupId, keyword: query, page, pageSize: 20 }
  const base = useResource(resourceKeys.knowledgeBase(baseId), (signal) => getKnowledgeBase(baseId, signal))
  const list = useResource(
    resourceKeys.knowledgeDocuments(baseId, parameters),
    (signal) => listKnowledgeDocuments(baseId, parameters, signal),
    { staleTime: 0, keepPreviousData: true, refetchInterval: (data) => data?.documents.some((document) => document.status === "queued" || document.status === "running") ? 2000 : false },
  )
  const groups = base.data?.groups.flatMap((group) => [group, ...group.children]) ?? []
  const group = groups.find((item) => item.id === groupId)
  const listPath = `/knowledge-bases/${baseId}/groups/${groupId}/documents`
  const scroll = useListScrollRestore(
    `${listPath}${location.search}`,
    Boolean(list.data && !list.isPlaceholderData && base.data),
  )
  const [action, setAction] = useState<DocumentAction | null>(null)
  const [retrievalOpen, setRetrievalOpen] = useState(false)
  const retrievalTrigger = useRef<HTMLButtonElement>(null)
  const pages = Math.max(1, Math.ceil((list.data?.page.total ?? 0) / 20))
  useEffect(() => {
    if (list.data && !list.isPlaceholderData && page > pages)
      setParameters({ page: pages === 1 ? null : String(pages) }, true)
  }, [list.data, list.isPlaceholderData, page, pages, setParameters])
  return (
    <>
      <PageHeader
        title={
          [base.data?.name, group?.isDefault ? t("group.default") : group?.name].filter(Boolean).join(" · ") ||
          t("documents.title")
        }
        description={t("documents.description")}
      >
        {list.data && !list.error && (
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
            <KnowledgeDocumentUpload baseId={baseId} groupId={groupId} />
          </>
        )}
      </PageHeader>
      <ListToolbar>
        <ListToolbarSearch
          value={search}
          aria-label={t("documents.search")}
          onChange={(event) => setSearch(event.target.value)}
        />
        {search && (
          <ListToolbarReset
            onClick={() => {
              setSearch("")
              setParameters({ q: null, page: null })
            }}
          >
            {t("common:actions.clearFilters")}
          </ListToolbarReset>
        )}
      </ListToolbar>
      <PageContent ref={scroll.ref} onScroll={scroll.onScroll}>
        {!base.data || !list.data ? (
          <KnowledgeQAFeedback
            error={base.error ?? list.error}
            retry={() => void (base.error ? base.refresh() : list.refresh())}
          />
        ) : (
          <KnowledgeDocumentTable
            data={list.data}
            listPath={listPath}
            search={location.search}
            filtered={Boolean(query)}
            refreshing={list.isPlaceholderData}
            canMove={groups.length > 1}
            onAction={setAction}
            onPage={(value) => setParameters({ page: value === 1 ? null : String(value) })}
          />
        )}
      </PageContent>
      {action && base.data && (
        <KnowledgeDocumentActions base={base.data} action={action} onClose={() => setAction(null)} />
      )}
      <KnowledgeRetrievalSheet open={retrievalOpen} onOpenChange={setRetrievalOpen} knowledgeBaseId={baseId} category={KnowledgeBaseCategory.KnowledgeBaseCategoryStandard} triggerRef={retrievalTrigger} />
    </>
  )
}
