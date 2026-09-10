/** 文档表格展示元数据、创建时间及固定操作栏。 */
import { useRef, useState } from "react"
import { toast } from "sonner"
import { retryKnowledgeDocument, isApiError } from "@/api"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { apiErrorMessage } from "@/lib/form-errors"
import { Link, useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import type { KnowledgeDocumentData, KnowledgeDocumentListData } from "@/api"
import { PageControls } from "@/components/page-controls"
import { ResourceTable } from "@/components/resource-table"
import { SelectableText } from "@/components/selectable-text"
import { Button } from "@/components/ui/button"
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { useDateTime } from "@/hooks/use-date-time"
import { formatFileSize } from "@/lib/file-size"
import type { DocumentAction } from "./knowledge-document-actions"
import { KnowledgeDocumentStatus } from "./knowledge-document-status"

/** 显示文档列表，操作列固定在最右侧。 */
export function KnowledgeDocumentTable({
  data,
  listPath,
  search,
  filtered,
  refreshing,
  canMove,
  onAction,
  onPage,
}: {
  data: KnowledgeDocumentListData
  listPath: string
  search: string
  filtered: boolean
  refreshing: boolean
  canMove: boolean
  onAction: (action: DocumentAction) => void
  onPage: (page: number) => void
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const { formatDateTime } = useDateTime()
  const { knowledgeBaseId = "" } = useParams()
  const invalidate = useResourceInvalidator()
  const navigate = useNavigate()
  const [retryingIDs, setRetryingIDs] = useState<ReadonlySet<string>>(new Set())
  // 记录每行三点菜单按钮，供关闭对话框后恢复焦点。
  const triggers = useRef(new Map<string, HTMLButtonElement>())

  /** 提交重试并在结束后刷新列表和详情中的文档状态。 */
  async function retryDocument(document: KnowledgeDocumentData) {
    setRetryingIDs((current) => new Set(current).add(document.id))
    try {
      await retryKnowledgeDocument(knowledgeBaseId, document.id)
    } catch (error) {
      if (!recoverSession(error, navigate)) toast.error(isApiError(error) ? apiErrorMessage(error) : t("documents.retryFailed"))
    } finally {
      await Promise.all([
        invalidate(resourceKeys.knowledgeDocuments(knowledgeBaseId)),
        invalidate(resourceKeys.knowledgeDocument(knowledgeBaseId, document.id)),
      ])
      setRetryingIDs((current) => {
        const next = new Set(current)
        next.delete(document.id)
        return next
      })
    }
  }

  return (
    <div className="overflow-hidden rounded-lg border bg-card" aria-busy={refreshing}>
      <ResourceTable
        columns={[
          {
            key: "name",
            header: t("documents.columns.name"),
            cellClassName: "max-w-80 truncate font-medium",
            cell: (document) => (
              <SelectableText title={document.name}>
                {document.name}
              </SelectableText>
            ),
          },
          {
            key: "type",
            header: t("documents.columns.type"),
            cell: (document) => document.format.slice(1).toUpperCase(),
          },
          {
            key: "size",
            header: t("documents.columns.size"),
            cellClassName: "whitespace-nowrap tabular-nums",
            cell: (document) => formatFileSize(document.byteSize),
          },
          {
            key: "status",
            header: t("documents.columns.status"),
            cell: (document) => <KnowledgeDocumentStatus document={document} />,
          },
          {
            key: "createdAt",
            header: t("documents.columns.createdAt"),
            cellClassName: "whitespace-nowrap text-muted-foreground",
            cell: (document) => formatDateTime(document.createdAt),
          },
        ]}
        rows={data.documents}
        rowKey={(document) => document.id}
        empty={t(filtered ? "documents.filteredEmpty" : "documents.empty")}
        actions={(document) => ({
          primary: (
            <Button variant="outline" size="sm" asChild>
              <Link to={`${listPath}/${document.id}${search}`}>
                {t("common:actions.view")}
              </Link>
            </Button>
          ),
          menuLabel: t("documents.more", { name: document.name }),
          menuTriggerRef: (node) => {
            if (node) triggers.current.set(document.id, node)
            else triggers.current.delete(document.id)
          },
          menu: (
            <>
              <DropdownMenuItem
                disabled={retryingIDs.has(document.id)}
                onSelect={() => void retryDocument(document)}
              >
                {t("common:actions.retry")}
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!canMove}
                onSelect={() =>
                  onAction({
                    document,
                    kind: "move",
                    trigger: triggers.current.get(document.id) ?? null,
                  })
                }
              >
                {t("documents.move")}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  onAction({
                    document,
                    kind: "delete",
                    trigger: triggers.current.get(document.id) ?? null,
                  })
                }
              >
                {t("common:actions.delete")}
              </DropdownMenuItem>
            </>
          ),
        })}
      />
      <PageControls page={data.page} disabled={refreshing} onPageChange={onPage} />
    </div>
  )
}
