/** 文档表格展示元数据、创建时间及固定操作栏。 */
import { useRef, useState } from "react"
import { MoreHorizontalIcon } from "lucide-react"
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
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { useDateTime } from "@/hooks/use-date-time"
import { formatFileSize } from "@/lib/file-size"
import type { DocumentAction } from "./knowledge-document-actions"
import { KnowledgeDocumentStatus } from "./knowledge-document-status"

/** 显示文档列表，保持详情和三点菜单的位置一致。 */
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
  return (
    <div className="overflow-hidden rounded-lg border bg-card" aria-busy={refreshing}>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("documents.columns.name")}</TableHead>
            <TableHead>{t("documents.columns.type")}</TableHead>
            <TableHead>{t("documents.columns.size")}</TableHead>
            <TableHead>{t("documents.columns.status")}</TableHead>
            <TableHead>{t("documents.columns.createdAt")}</TableHead>
            <TableHead className="w-px">{t("common:table.actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {!data.documents.length ? (
            <TableRow>
              <TableCell colSpan={6} className="h-32 text-center text-muted-foreground">
                {t(filtered ? "documents.filteredEmpty" : "documents.empty")}
              </TableCell>
            </TableRow>
          ) : (
            data.documents.map((document: KnowledgeDocumentData) => (
              <KnowledgeDocumentRow
                key={document.id}
                document={document}
                listPath={listPath}
                search={search}
                canMove={canMove}
                onAction={onAction}
              />
            ))
          )}
        </TableBody>
      </Table>
      <PageControls page={data.page} disabled={refreshing} onPageChange={onPage} />
    </div>
  )
}

/** 保留每行菜单触发点，关闭对话框后恢复键盘焦点。 */
function KnowledgeDocumentRow({
  document,
  listPath,
  search,
  canMove,
  onAction,
}: {
  document: KnowledgeDocumentData
  listPath: string
  search: string
  canMove: boolean
  onAction: (action: DocumentAction) => void
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const { formatDateTime } = useDateTime()
  const trigger = useRef<HTMLButtonElement>(null)
  const { knowledgeBaseId = "" } = useParams()
  const invalidate = useResourceInvalidator()
  const navigate = useNavigate()
  const [retrying, setRetrying] = useState(false)

  /** 提交重试并刷新文档状态，保留列表位置。 */
  async function retry() {
    setRetrying(true)
    try {
      await retryKnowledgeDocument(knowledgeBaseId, document.id)
    } catch (error) {
      if (!recoverSession(error, navigate)) toast.error(isApiError(error) ? apiErrorMessage(error) : t("documents.retryFailed"))
    } finally {
      // 连接失败也会更新服务端状态，请求结束后统一重新读取。
      await Promise.all([
        invalidate(resourceKeys.knowledgeDocuments(knowledgeBaseId)),
        invalidate(resourceKeys.knowledgeDocument(knowledgeBaseId, document.id)),
      ])
      setRetrying(false)
    }
  }
  return (
    <TableRow>
      <TableCell className="max-w-80 truncate font-medium" title={document.name}>
        {document.name}
      </TableCell>
      <TableCell>{document.format.slice(1).toUpperCase()}</TableCell>
      <TableCell className="whitespace-nowrap tabular-nums">{formatFileSize(document.byteSize)}</TableCell>
      <TableCell>
        <KnowledgeDocumentStatus document={document} />
      </TableCell>
      <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(document.createdAt)}</TableCell>
      <TableCell className="whitespace-nowrap">
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to={`${listPath}/${document.id}${search}`}>{t("common:actions.view")}</Link>
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                ref={trigger}
                variant="ghost"
                size="icon-sm"
                aria-label={t("documents.more", { name: document.name })}
              >
                <MoreHorizontalIcon />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem disabled={retrying} onSelect={() => void retry()}>
                {t("common:actions.retry")}
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!canMove}
                onSelect={() => onAction({ document, kind: "move", trigger: trigger.current })}
              >
                {t("documents.move")}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() => onAction({ document, kind: "delete", trigger: trigger.current })}
              >
                {t("common:actions.delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </TableCell>
    </TableRow>
  )
}
