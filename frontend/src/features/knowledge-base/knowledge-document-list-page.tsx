/** 外部知识库文档只读列表页。 */
import { useEffect } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, useParams } from "react-router"

import {
  KnowledgeDocumentStatus,
  getKnowledgeBase,
  isApiError,
  listKnowledgeDocuments,
  type KnowledgeDocumentStatusId,
} from "@/api"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
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

const documentPageSize = 20
const documentStatuses = [
  KnowledgeDocumentStatus.KnowledgeDocumentStatusQueued,
  KnowledgeDocumentStatus.KnowledgeDocumentStatusProcessing,
  KnowledgeDocumentStatus.KnowledgeDocumentStatusPaused,
  KnowledgeDocumentStatus.KnowledgeDocumentStatusError,
  KnowledgeDocumentStatus.KnowledgeDocumentStatusReady,
  KnowledgeDocumentStatus.KnowledgeDocumentStatusDisabled,
  KnowledgeDocumentStatus.KnowledgeDocumentStatusArchived,
] satisfies KnowledgeDocumentStatusId[]

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

/** 显示指定外部知识库的一页文档。 */
export function KnowledgeDocumentListPage() {
  const { t } = useTranslation("knowledgeBase")
  const { formatDateTime } = useDateTime()
  const { knowledgeBaseId = "" } = useParams()
  const {
    searchParams,
    setParameters,
    query: keyword,
    search,
    setSearch,
  } = useListSearchParams()
  const status = optionalWailsEnum(
    KnowledgeDocumentStatus,
    searchParams.get("status"),
  )
  const parsedPage = Number(searchParams.get("page") ?? "1")
  const currentPage =
    Number.isSafeInteger(parsedPage) && parsedPage > 0 ? parsedPage : 1

  const detail = useResource(
    resourceKeys.knowledgeBase(knowledgeBaseId),
    (signal) => getKnowledgeBase(knowledgeBaseId, signal),
    { enabled: knowledgeBaseId !== "" },
  )
  const list = useResource(
    resourceKeys.knowledgeDocuments(knowledgeBaseId, {
      keyword,
      status,
      page: currentPage,
      pageSize: documentPageSize,
    }),
    (signal) =>
      listKnowledgeDocuments(
        knowledgeBaseId,
        {
          keyword,
          status,
          page: currentPage,
          pageSize: documentPageSize,
        },
        signal,
      ),
    {
      enabled:
        knowledgeBaseId !== "" &&
        Boolean(detail.data?.integrationConnectionId),
      staleTime: 0,
    },
  )
  const error = detail.error ?? list.error
  const showLoading =
    detail.loading ||
    list.loading ||
    (Boolean(error) && (detail.refreshing || list.refreshing))
  const documents = list.data?.documents ?? []
  const page = list.data?.page ?? {
    number: currentPage,
    size: documentPageSize,
    total: 0,
  }
  const totalPages = Math.max(1, Math.ceil(page.total / page.size))

  /** 把超出总页数的地址收回最后一页。 */
  useEffect(() => {
    if (!list.data || currentPage <= totalPages) return
    setParameters(
      { page: totalPages === 1 ? null : String(totalPages) },
      true,
    )
  }, [currentPage, list.data, setParameters, totalPages])

  /** 切换文档页码。 */
  function changePage(nextPage: number) {
    setParameters({ page: nextPage <= 1 ? null : String(nextPage) })
  }

  if (detail.data?.integrationConnectionId === "") {
    return <Navigate replace to={`/knowledge-bases/${knowledgeBaseId}`} />
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={detail.data?.name ?? t("documents.title")} />
      <ListToolbar>
        <ListToolbarSearch
          value={search}
          aria-label={t("documents.filters.search")}
          onChange={(event) => setSearch(event.target.value)}
        />
        <ListToolbarFilter
          label={t("documents.filters.status")}
          allLabel={t("documents.filters.allStatuses")}
          value={status ?? ""}
          options={documentStatuses.map((documentStatus) => ({
            value: documentStatus,
            label: t(`documents.status.${documentStatus}`),
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
            {t("documents.filters.clear")}
          </ListToolbarReset>
        ) : null}
      </ListToolbar>
      <PageContent>
        {showLoading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("documents.loading")}
          </div>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {isApiError(error)
                ? apiErrorMessage(error)
                : t("documents.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() =>
                void (detail.error ? detail.refresh() : list.refresh())
              }
            >
              {t("retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("documents.columns.name")}</TableHead>
                  <TableHead>{t("documents.columns.status")}</TableHead>
                  <TableHead>{t("documents.columns.createdAt")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {documents.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={3}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {keyword || status
                        ? t("documents.filteredEmpty")
                        : t("documents.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  documents.map((document) => (
                    <TableRow key={document.id}>
                      <TableCell className="font-medium">
                        <SelectableText>{document.name}</SelectableText>
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          variant={statusVariant(document.status)}
                          showDot={false}
                        >
                          {t(`documents.status.${document.status}`)}
                        </StatusBadge>
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {document.createdAt
                          ? formatDateTime(document.createdAt)
                          : "—"}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
            <div className="flex items-center justify-between border-t px-4 py-3 text-sm text-muted-foreground">
              <span>
                {t("documents.pagination.total", { count: page.total })}
              </span>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page.number <= 1}
                  onClick={() => changePage(page.number - 1)}
                >
                  {t("documents.pagination.previous")}
                </Button>
                <span>
                  {t("documents.pagination.page", {
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
                  {t("documents.pagination.next")}
                </Button>
              </div>
            </div>
          </div>
        )}
      </PageContent>
    </div>
  )
}
