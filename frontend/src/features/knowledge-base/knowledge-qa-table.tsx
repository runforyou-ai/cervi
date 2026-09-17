/** 展示问答列表、索引状态和分页操作。 */
import { useState } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"
import {
  isApiError,
  retryKnowledgeQAEntry,
  type KnowledgeQAListData,
  type KnowledgeQASummaryData,
} from "@/api"
import { PageControls } from "@/components/page-controls"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { KnowledgeIndexStatus } from "@/features/knowledge-base/knowledge-index-status"

/** 展示问答内容摘要、索引状态、创建时间和分页按钮。 */
export function KnowledgeQATable({
  knowledgeBaseId,
  data,
  loading,
  listPath,
  search,
  filtered,
  onDelete,
  onPageChange,
}: {
  knowledgeBaseId: string
  data: KnowledgeQAListData
  loading: boolean
  listPath: string
  search: string
  filtered: boolean
  onDelete: (entry: KnowledgeQASummaryData) => void
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [retryingIDs, setRetryingIDs] = useState<ReadonlySet<string>>(new Set())

  /** 提交重新索引并在结束后刷新列表中的状态。 */
  async function retryEntry(entry: KnowledgeQASummaryData) {
    setRetryingIDs((current) => new Set(current).add(entry.id))
    try {
      await retryKnowledgeQAEntry(knowledgeBaseId, entry.id)
    } catch (error) {
      if (!recoverSession(error, navigate))
        toast.error(isApiError(error) ? apiErrorMessage(error) : t("qa.retryFailed"))
    } finally {
      await invalidate(resourceKeys.knowledgeQAEntries(knowledgeBaseId))
      setRetryingIDs((current) => {
        const next = new Set(current)
        next.delete(entry.id)
        return next
      })
    }
  }

  return (
    <div className="overflow-hidden rounded-lg border bg-card" aria-busy={loading}>
      <Table className="min-w-[960px] table-fixed">
        {/* 固定辅助列宽，标准问题和答案均分剩余空间。 */}
        <colgroup>
          <col />
          <col className="w-32" />
          <col />
          <col className="w-24" />
          <col className="w-48" />
          <col className="w-32" />
        </colgroup>
        <TableHeader>
          <TableRow>
            <TableHead>{t("qa.question")}</TableHead>
            <TableHead>{t("qa.similarQuestions")}</TableHead>
            <TableHead>{t("qa.answer")}</TableHead>
            <TableHead>{t("qa.status")}</TableHead>
            <TableHead>{t("qa.createdAt")}</TableHead>
            <TableHead>
              {t("common:table.actions")}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.entries.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={6}
                className="h-32 text-center text-muted-foreground"
              >
                {filtered ? t("qa.filteredEmpty") : t("qa.empty")}
              </TableCell>
            </TableRow>
          ) : (
            data.entries.map((entry) => (
              <KnowledgeQARow
                key={entry.id}
                entry={entry}
                editPath={`${listPath}/${entry.id}/edit${search}`}
                retrying={retryingIDs.has(entry.id)}
                onRetry={() => void retryEntry(entry)}
                onDelete={() => onDelete(entry)}
              />
            ))
          )}
        </TableBody>
      </Table>
      <PageControls
        page={data.page}
        disabled={loading}
        totalLabel={t("qa.total", { count: data.page.total })}
        onPageChange={onPageChange}
      />
    </div>
  )
}

/** 展示一条问答及其编辑、重试和删除入口。 */
function KnowledgeQARow({
  entry,
  editPath,
  retrying,
  onRetry,
  onDelete,
}: {
  entry: KnowledgeQASummaryData
  editPath: string
  retrying: boolean
  onRetry: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const { formatDateTime } = useDateTime()
  return (
    <TableRow>
      <TableCell className="whitespace-pre-wrap break-words font-medium">
        {entry.question}
      </TableCell>
      <TableCell>
        {entry.similarQuestions.length > 0 ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span
                tabIndex={0}
                className="cursor-help outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {entry.similarQuestions.length}
              </span>
            </TooltipTrigger>
            <TooltipContent side="bottom" sideOffset={4} className="max-w-sm">
              <ul className="grid max-h-64 gap-1 overflow-y-auto text-left">
                {entry.similarQuestions.map((question) => (
                  <li key={question} className="whitespace-pre-wrap break-words">
                    {question}
                  </li>
                ))}
              </ul>
            </TooltipContent>
          </Tooltip>
        ) : 0}
      </TableCell>
      <TableCell className="text-muted-foreground">
        <span className="block truncate">{entry.answer}</span>
      </TableCell>
      <TableCell>
        <KnowledgeIndexStatus status={entry.status} failureMessage={entry.failureMessage} />
      </TableCell>
      <TableCell className="whitespace-nowrap text-muted-foreground">
        {formatDateTime(entry.createdAt)}
      </TableCell>
      <TableCell className="whitespace-nowrap">
        <div className="inline-flex items-center gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to={editPath}>{t("common:actions.edit")}</Link>
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("qa.more", {
                  question: entry.question,
                })}
              >
                <MoreHorizontalIcon />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem disabled={retrying} onSelect={onRetry}>
                {t("common:actions.retry")}
              </DropdownMenuItem>
              <DropdownMenuItem destructive onSelect={onDelete}>
                {t("common:actions.delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </TableCell>
    </TableRow>
  )
}
