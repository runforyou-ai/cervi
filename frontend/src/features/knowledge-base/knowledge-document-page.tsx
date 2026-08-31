/** 外部知识文档详情与分段只读页面。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useParams } from "react-router"

import {
  KnowledgeBaseCategory,
  KnowledgeDocumentSegmentIndexStatus,
  KnowledgeDocumentStatus,
  getKnowledgeBase,
  getKnowledgeDocument,
  isApiError,
  listKnowledgeDocumentSegments,
  type KnowledgeDocumentSegmentIndexStatusId,
  type KnowledgeDocumentStatusId,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { PageBack } from "@/components/page-back"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { SelectableText } from "@/components/selectable-text"
import { StatusBadge } from "@/components/status-badge"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useDateTime } from "@/hooks/use-date-time"
import { useListSearchParams } from "@/hooks/use-list-search-params"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { optionalWailsEnum } from "@/lib/wails-enum"

const segmentPageSize = 20
const segmentIndexStatuses = [
  KnowledgeDocumentSegmentIndexStatus.KnowledgeDocumentSegmentIndexStatusWaiting,
  KnowledgeDocumentSegmentIndexStatus.KnowledgeDocumentSegmentIndexStatusIndexing,
  KnowledgeDocumentSegmentIndexStatus.KnowledgeDocumentSegmentIndexStatusCompleted,
  KnowledgeDocumentSegmentIndexStatus.KnowledgeDocumentSegmentIndexStatusError,
  KnowledgeDocumentSegmentIndexStatus.KnowledgeDocumentSegmentIndexStatusPaused,
  KnowledgeDocumentSegmentIndexStatus.KnowledgeDocumentSegmentIndexStatusResegment,
] satisfies KnowledgeDocumentSegmentIndexStatusId[]

/** 选择知识文档状态的展示样式。 */
function statusVariant(status: KnowledgeDocumentStatusId) {
  if (status === KnowledgeDocumentStatus.KnowledgeDocumentStatusReady) {
    return "success" as const
  }
  if (status === KnowledgeDocumentStatus.KnowledgeDocumentStatusError) {
    return "destructive" as const
  }
  return "muted" as const
}

/** 选择知识文档分段索引状态的展示样式。 */
function segmentIndexStatusVariant(
  status: KnowledgeDocumentSegmentIndexStatusId,
) {
  if (
    status ===
    KnowledgeDocumentSegmentIndexStatus.KnowledgeDocumentSegmentIndexStatusCompleted
  ) {
    return "success" as const
  }
  if (
    status ===
    KnowledgeDocumentSegmentIndexStatus.KnowledgeDocumentSegmentIndexStatusError
  ) {
    return "destructive" as const
  }
  return "muted" as const
}

/** 显示指定外部知识文档的详情与分段。 */
export function KnowledgeDocumentPage() {
  const { t } = useTranslation("knowledgeBase")
  const { formatDateTime } = useDateTime()
  const { knowledgeBaseId = "", documentId = "" } = useParams()
  const {
    searchParams,
    setParameters,
    query: keyword,
    search,
    setSearch,
  } = useListSearchParams()
  const status = optionalWailsEnum(
    KnowledgeDocumentSegmentIndexStatus,
    searchParams.get("status"),
  )
  const parsedPage = Number(searchParams.get("page") ?? "1")
  const currentPage =
    Number.isSafeInteger(parsedPage) && parsedPage > 0 ? parsedPage : 1

  const knowledgeBase = useResource(
    resourceKeys.knowledgeBase(knowledgeBaseId),
    (signal) => getKnowledgeBase(knowledgeBaseId, signal),
    { enabled: knowledgeBaseId !== "" },
  )
  const externalEnabled = Boolean(
    knowledgeBase.data?.integrationConnectionId,
  )
  const document = useResource(
    resourceKeys.knowledgeDocument(knowledgeBaseId, documentId),
    (signal) =>
      getKnowledgeDocument(knowledgeBaseId, documentId, signal),
    {
      enabled:
        knowledgeBaseId !== "" && documentId !== "" && externalEnabled,
      staleTime: 0,
    },
  )
  const segments = useResource(
    resourceKeys.knowledgeDocumentSegments(knowledgeBaseId, documentId, {
      keyword,
      status,
      page: currentPage,
      pageSize: segmentPageSize,
    }),
    (signal) =>
      listKnowledgeDocumentSegments(
        knowledgeBaseId,
        documentId,
        { keyword, status, page: currentPage, pageSize: segmentPageSize },
        signal,
      ),
    {
      enabled:
        knowledgeBaseId !== "" && documentId !== "" && externalEnabled,
      staleTime: 0,
    },
  )
  const error = knowledgeBase.error ?? document.error ?? segments.error
  const showLoading =
    knowledgeBase.loading ||
    document.loading ||
    segments.loading ||
    (Boolean(error) &&
      (knowledgeBase.refreshing || document.refreshing || segments.refreshing))
  const page = segments.data?.page ?? {
    number: currentPage,
    size: segmentPageSize,
    total: 0,
  }
  const totalPages = Math.max(1, Math.ceil(page.total / page.size))
  const isQA =
    knowledgeBase.data?.category ===
    KnowledgeBaseCategory.KnowledgeBaseCategoryQA
  const columnCount = isQA ? 7 : 6

  /** 把超出总页数的地址收回最后一页。 */
  useEffect(() => {
    if (!segments.data || currentPage <= totalPages) return
    setParameters(
      { page: totalPages === 1 ? null : String(totalPages) },
      true,
    )
  }, [currentPage, segments.data, setParameters, totalPages])

  /** 切换文档分段页码。 */
  function changePage(nextPage: number) {
    setParameters({ page: nextPage <= 1 ? null : String(nextPage) })
  }

  /** 重试第一个失败的数据请求。 */
  function retry() {
    if (knowledgeBase.error) {
      void knowledgeBase.refresh()
    } else if (document.error) {
      void document.refresh()
    } else {
      void segments.refresh()
    }
  }

  if (knowledgeBase.data?.integrationConnectionId === "") {
    return <Navigate replace to={`/knowledge-bases/${knowledgeBaseId}`} />
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={document.data?.name ?? t("documentDetail.title")}
      >
        <PageBack to={`/knowledge-bases/${knowledgeBaseId}/documents`} />
      </PageHeader>
      {document.data ? (
        <dl className="grid shrink-0 gap-x-8 gap-y-3 border-b px-4 py-4 text-sm sm:grid-cols-3 sm:px-6 xl:grid-cols-5">
          <div>
            <dt className="text-muted-foreground">
              {t("documentDetail.metadata.status")}
            </dt>
            <dd className="mt-1">
              <StatusBadge
                variant={statusVariant(document.data.status)}
                showDot={false}
              >
                {t(`documents.status.${document.data.status}`)}
              </StatusBadge>
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">
              {t("documentDetail.metadata.wordCount")}
            </dt>
            <dd className="mt-1 font-medium">
              {document.data.wordCount ?? "—"}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">
              {t("documentDetail.metadata.hitCount")}
            </dt>
            <dd className="mt-1 font-medium">{document.data.hitCount}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">
              {t(
                keyword || status
                  ? "documentDetail.metadata.matchedSegmentCount"
                  : "documentDetail.metadata.segmentCount",
              )}
            </dt>
            <dd className="mt-1 font-medium">
              {segments.data ? page.total : "—"}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">
              {t("documentDetail.metadata.createdAt")}
            </dt>
            <dd className="mt-1 whitespace-nowrap font-medium">
              {document.data.createdAt
                ? formatDateTime(document.data.createdAt)
                : "—"}
            </dd>
          </div>
        </dl>
      ) : null}
      {document.data ? (
        <ListToolbar>
          <ListToolbarSearch
            value={search}
            aria-label={t("documentDetail.segments.filters.search")}
            onChange={(event) => setSearch(event.target.value)}
          />
          <ListToolbarFilter
            label={t("documentDetail.segments.filters.status")}
            allLabel={t("documentDetail.segments.filters.allStatuses")}
            value={status ?? ""}
            options={segmentIndexStatuses.map((indexStatus) => ({
              value: indexStatus,
              label: t(`documentDetail.segments.status.${indexStatus}`),
            }))}
            onValueChange={(value) =>
              setParameters({ status: value || null, page: null })
            }
          />
          {search || status ? (
            <ListToolbarReset
              onClick={() => {
                setSearch("")
                setParameters({ q: null, status: null, page: null })
              }}
            >
              {t("documentDetail.segments.filters.clear")}
            </ListToolbarReset>
          ) : null}
        </ListToolbar>
      ) : null}
      <PageContent>
        {showLoading ? (
          <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
            {t("documentDetail.loading")}
          </LoadingIndicator>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {isApiError(error)
                ? apiErrorMessage(error)
                : t("documentDetail.loadError")}
            </p>
            <Button className="mt-4" variant="outline" onClick={retry}>
              {t("retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-x-auto rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="w-16">
                    {t("documentDetail.segments.columns.position")}
                  </TableHead>
                  <TableHead>
                    {t(
                      isQA
                        ? "documentDetail.segments.columns.question"
                        : "documentDetail.segments.columns.content",
                    )}
                  </TableHead>
                  {isQA ? (
                    <TableHead>
                      {t("documentDetail.segments.columns.answer")}
                    </TableHead>
                  ) : null}
                  <TableHead>
                    {t("documentDetail.segments.columns.wordCount")}
                  </TableHead>
                  <TableHead>
                    {t("documentDetail.segments.columns.hitCount")}
                  </TableHead>
                  <TableHead>
                    {t("documentDetail.segments.columns.indexStatus")}
                  </TableHead>
                  <TableHead>
                    {t("documentDetail.segments.columns.createdAt")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(segments.data?.segments ?? []).length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={columnCount}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {keyword || status
                        ? t("documentDetail.segments.filteredEmpty")
                        : t("documentDetail.segments.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  segments.data?.segments.map((segment) => (
                    <TableRow key={segment.id}>
                      <TableCell className="align-top font-medium">
                        {segment.position}
                      </TableCell>
                      <TableCell className="min-w-72 max-w-2xl align-top">
                        <SelectableText className="block whitespace-pre-wrap break-words leading-6">
                          {segment.content || "—"}
                        </SelectableText>
                      </TableCell>
                      {isQA ? (
                        <TableCell className="min-w-72 max-w-2xl align-top">
                          <SelectableText className="block whitespace-pre-wrap break-words leading-6">
                            {segment.answer || "—"}
                          </SelectableText>
                        </TableCell>
                      ) : null}
                      <TableCell className="align-top">
                        {segment.wordCount}
                      </TableCell>
                      <TableCell className="align-top">
                        {segment.hitCount}
                      </TableCell>
                      <TableCell className="align-top">
                        <StatusBadge
                          variant={segmentIndexStatusVariant(
                            segment.indexStatus,
                          )}
                          showDot={false}
                        >
                          {t(
                            `documentDetail.segments.status.${segment.indexStatus}`,
                          )}
                        </StatusBadge>
                      </TableCell>
                      <TableCell className="align-top whitespace-nowrap text-muted-foreground">
                        {segment.createdAt
                          ? formatDateTime(segment.createdAt)
                          : "—"}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
            <div className="flex items-center justify-between gap-4 border-t px-4 py-3 text-sm text-muted-foreground">
              <span>
                {t("documentDetail.segments.pagination.total", {
                  count: page.total,
                })}
              </span>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page.number <= 1}
                  onClick={() => changePage(page.number - 1)}
                >
                  {t("documentDetail.segments.pagination.previous")}
                </Button>
                <span>
                  {t("documentDetail.segments.pagination.page", {
                    current: page.number,
                    total: totalPages,
                  })}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page.number >= totalPages}
                  onClick={() => changePage(page.number + 1)}
                >
                  {t("documentDetail.segments.pagination.next")}
                </Button>
              </div>
            </div>
          </div>
        )}
      </PageContent>
    </div>
  )
}
