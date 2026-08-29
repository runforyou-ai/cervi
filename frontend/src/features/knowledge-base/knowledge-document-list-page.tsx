/** 外部知识库文档只读列表页。 */
import { useEffect } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, useParams, useSearchParams } from "react-router"

import {
  KnowledgeDocumentStatus,
  getKnowledgeBase,
  isApiError,
  listKnowledgeDocuments,
  type KnowledgeDocumentStatusId,
} from "@/api"
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
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"

const documentPageSize = 20

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
  const [searchParams, setSearchParams] = useSearchParams()
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
      page: currentPage,
      pageSize: documentPageSize,
    }),
    (signal) =>
      listKnowledgeDocuments(
        knowledgeBaseId,
        { page: currentPage, pageSize: documentPageSize },
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
    const next = new URLSearchParams(searchParams)
    if (totalPages === 1) {
      next.delete("page")
    } else {
      next.set("page", String(totalPages))
    }
    setSearchParams(next, { replace: true })
  }, [currentPage, list.data, searchParams, setSearchParams, totalPages])

  /** 切换文档页码。 */
  function changePage(nextPage: number) {
    const next = new URLSearchParams(searchParams)
    if (nextPage <= 1) {
      next.delete("page")
    } else {
      next.set("page", String(nextPage))
    }
    setSearchParams(next)
  }

  if (detail.data?.integrationConnectionId === "") {
    return <Navigate replace to={`/knowledge-bases/${knowledgeBaseId}`} />
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={detail.data?.name ?? t("documents.title")} />
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
                      {t("documents.empty")}
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
