/** 知识库分组内容列表（文档、问答）共用的读取、页头、搜索和召回测试外壳。 */
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

/** 分组列表的读取参数。 */
export type KnowledgeGroupListParameters = {
  groupId: string
  keyword: string
  page: number
  pageSize: number
}

/** 按知识库和分组切换列表实例，隔离弹窗、上传批次和滚动恢复状态。 */
export function KnowledgeGroupRoute({
  children,
}: {
  children: (ids: { knowledgeBaseId: string; groupId: string }) => ReactNode
}) {
  const { knowledgeBaseId = "", groupId = "" } = useParams()
  return (
    <Fragment key={`${knowledgeBaseId}/${groupId}`}>
      {children({ knowledgeBaseId, groupId })}
    </Fragment>
  )
}

/** 读取知识库与当前分组的一页内容；有条目在排队或处理中时每 2 秒刷新，页码越界时回到最后一页。 */
export function useKnowledgeGroupList<T extends { page: PageInfo }>({
  knowledgeBaseId,
  groupId,
  section,
  listKey,
  load,
  processing,
}: {
  knowledgeBaseId: string
  groupId: string
  section: "documents" | "qa"
  listKey: (parameters: KnowledgeGroupListParameters) => QueryKey
  load: (parameters: KnowledgeGroupListParameters, signal: AbortSignal) => Promise<T>
  processing: (data: T) => boolean
}) {
  const { t } = useTranslation("knowledgeBase")
  const location = useLocation()
  const listSearch = useListSearchParams()
  const { searchParams, query, setParameters } = listSearch
  const page = parseListPage(searchParams.get("page"))
  const parameters = { groupId, keyword: query, page, pageSize }
  const base = useResource(resourceKeys.knowledgeBase(knowledgeBaseId), (signal) =>
    getKnowledgeBase(knowledgeBaseId, signal),
  )
  const list = useResource(listKey(parameters), (signal) => load(parameters, signal), {
    staleTime: 0,
    keepPreviousData: true,
    refetchInterval: (data) => (data && processing(data) ? 2000 : false),
  })
  const groups =
    base.data?.groups.flatMap((group) => [group, ...group.children]) ?? []
  const group = groups.find((item) => item.id === groupId)
  const listPath = `/knowledge-bases/${knowledgeBaseId}/groups/${groupId}/${section}`
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
    groups,
    listPath,
    scroll,
    /** 页头标题：知识库名称 · 分组名称。 */
    title: [base.data?.name, group?.isDefault ? t("group.default") : group?.name]
      .filter(Boolean)
      .join(" · "),
    /** 切换页码，第一页不写入地址。 */
    changePage: (value: number) =>
      setParameters({ page: value === 1 ? null : String(value) }),
  }
}

/** 渲染分组列表的页头（召回测试加业务操作）、搜索工具栏、读取状态和召回测试面板。 */
export function KnowledgeGroupListShell({
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
  list: ReturnType<typeof useKnowledgeGroupList>
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
