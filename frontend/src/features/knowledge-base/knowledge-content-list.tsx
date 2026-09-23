/** 知识库文档、问答列表共用的读取、页头、搜索和召回测试外壳。 */
import { Fragment, useEffect, useRef, useState, type ReactNode } from "react"
import type { QueryKey } from "@tanstack/react-query"
import { SearchCheckIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useLocation, useParams } from "react-router"

import {
  getKnowledgeBase,
  type KnowledgeBaseCategoryId,
  type PageInfo,
} from "@/api"
import {
  ListToolbar,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageBackButton } from "@/components/page-back-button"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import { Button } from "@/components/ui/button"
import { useListScrollRestore } from "@/hooks/use-list-scroll-restore"
import { useListSearchParams } from "@/hooks/use-list-search-params"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { parseListPage } from "@/lib/list-page"
import { KnowledgeRetrievalSheet } from "./knowledge-retrieval-sheet"

const pageSize = 20

/** 知识库内容列表的读取参数。 */
export type KnowledgeContentListParameters = {
  keyword: string
  page: number
  pageSize: number
}

/** 按知识库切换列表实例，隔离弹窗、上传批次和滚动恢复状态。 */
export function KnowledgeContentRoute({
  children,
}: {
  children: (ids: { knowledgeBaseId: string }) => ReactNode
}) {
  const { knowledgeBaseId = "" } = useParams()
  return (
    <Fragment key={knowledgeBaseId}>
      {children({ knowledgeBaseId })}
    </Fragment>
  )
}

/** 读取知识库的一页内容，处理中的条目每 2 秒刷新，页码越界时回到最后一页。 */
export function useKnowledgeContentList<T extends { page: PageInfo }>({
  knowledgeBaseId,
  section,
  listKey,
  load,
  processing,
}: {
  knowledgeBaseId: string
  section: "documents" | "qa"
  listKey: (parameters: KnowledgeContentListParameters) => QueryKey
  load: (parameters: KnowledgeContentListParameters, signal: AbortSignal) => Promise<T>
  processing: (data: T) => boolean
}) {
  const location = useLocation()
  const listSearch = useListSearchParams()
  const { searchParams, query, setParameters } = listSearch
  const page = parseListPage(searchParams.get("page"))
  const parameters = { keyword: query, page, pageSize }
  const base = useResource(resourceKeys.knowledgeBase(knowledgeBaseId), (signal) =>
    getKnowledgeBase(knowledgeBaseId, signal),
  )
  const list = useResource(listKey(parameters), (signal) => load(parameters, signal), {
    staleTime: 0,
    keepPreviousData: true,
    refetchInterval: (data) => (data && processing(data) ? 2000 : false),
  })
  const listPath = `/knowledge-bases/${knowledgeBaseId}/${section}`
  const scroll = useListScrollRestore(
    `${listPath}${location.search}`,
    Boolean(list.data && !list.isPlaceholderData && base.data),
  )
  const pages = Math.max(1, Math.ceil((list.data?.page.total ?? 0) / pageSize))

  useEffect(() => {
    if (list.data && !list.isPlaceholderData && page > pages)
      setParameters({ page: pages === 1 ? null : String(pages) }, true)
  }, [list.data, list.isPlaceholderData, page, pages, setParameters])

  return {
    ...listSearch,
    base,
    list,
    listPath,
    scroll,
    title: base.data?.name,
    /** 切换页码，第一页不写入地址。 */
    changePage: (value: number) =>
      setParameters({ page: value === 1 ? null : String(value) }),
  }
}

/** 渲染知识库内容列表的页头、搜索工具栏、读取状态和召回测试面板。 */
export function KnowledgeContentListShell({
  list,
  knowledgeBaseId,
  category,
  fallbackTitle,
  description,
  searchLabel,
  errorMessage,
  actions,
  children,
}: {
  list: ReturnType<typeof useKnowledgeContentList>
  knowledgeBaseId: string
  category: KnowledgeBaseCategoryId
  fallbackTitle: string
  description: string
  searchLabel: string
  errorMessage: string
  actions: ReactNode
  children: ReactNode
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const [retrievalOpen, setRetrievalOpen] = useState(false)
  const retrievalTrigger = useRef<HTMLButtonElement>(null)
  const { search, setSearch, setParameters } = list

  return (
    <>
      <PageHeader title={list.title || fallbackTitle} description={description}>
        {list.list.data && !list.list.error ? (
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
            {actions}
          </>
        ) : null}
        <PageBackButton to="/knowledge-bases" />
      </PageHeader>
      <ListToolbar>
        <ListToolbarSearch
          value={search}
          aria-label={searchLabel}
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
      <PageContent ref={list.scroll.ref} onScroll={list.scroll.onScroll}>
        <ResourceContent resources={[list.base, list.list]} errorMessage={errorMessage}>
          {children}
        </ResourceContent>
      </PageContent>
      <KnowledgeRetrievalSheet
        open={retrievalOpen}
        onOpenChange={setRetrievalOpen}
        knowledgeBaseId={knowledgeBaseId}
        category={category}
        triggerRef={retrievalTrigger}
      />
    </>
  )
}
