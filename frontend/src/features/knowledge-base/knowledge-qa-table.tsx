/** 展示问答列表和分页操作。 */
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import type { KnowledgeQAListData, KnowledgeQASummaryData } from "@/api"
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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

/** 展示问答内容摘要、创建时间和分页按钮。 */
export function KnowledgeQATable({
  data,
  loading,
  listPath,
  search,
  filtered,
  onDelete,
  onPageChange,
}: {
  data: KnowledgeQAListData
  loading: boolean
  listPath: string
  search: string
  filtered: boolean
  onDelete: (entry: KnowledgeQASummaryData) => void
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation("knowledgeBase")
  const pageNumber = data.page.number
  const totalPages = Math.max(1, Math.ceil(data.page.total / data.page.size))
  return (
    <div className="overflow-hidden rounded-lg border" aria-busy={loading}>
      <Table className="min-w-[900px] table-fixed">
        {/* 固定辅助列宽，标准问题和答案均分剩余空间。 */}
        <colgroup>
          <col />
          <col className="w-32" />
          <col />
          <col className="w-48" />
          <col className="w-32" />
        </colgroup>
        <TableHeader>
          <TableRow>
            <TableHead>{t("qa.question")}</TableHead>
            <TableHead>{t("qa.similarQuestions")}</TableHead>
            <TableHead>{t("qa.answer")}</TableHead>
            <TableHead>{t("qa.createdAt")}</TableHead>
            <TableHead className="text-right">
              {t("documents.columns.actions")}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.entries.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={5}
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
                onDelete={() => onDelete(entry)}
              />
            ))
          )}
        </TableBody>
      </Table>
      <div className="flex items-center justify-between gap-3 border-t px-4 py-3 text-sm text-muted-foreground">
        <span>{t("qa.total", { count: data.page.total })}</span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={loading || pageNumber <= 1}
            onClick={() => onPageChange(pageNumber - 1)}
          >
            {t("documents.pagination.previous")}
          </Button>
          <span>
            {t("documents.pagination.page", {
              current: pageNumber,
              total: totalPages,
            })}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={loading || pageNumber >= totalPages}
            onClick={() => onPageChange(pageNumber + 1)}
          >
            {t("documents.pagination.next")}
          </Button>
        </div>
      </div>
    </div>
  )
}

/** 展示一条问答及其编辑和删除入口。 */
function KnowledgeQARow({
  entry,
  editPath,
  onDelete,
}: {
  entry: KnowledgeQASummaryData
  editPath: string
  onDelete: () => void
}) {
  const { t } = useTranslation("knowledgeBase")
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
      <TableCell className="whitespace-nowrap text-muted-foreground">
        {formatDateTime(entry.createdAt)}
      </TableCell>
      <TableCell className="text-right">
        <div className="flex items-center justify-end gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to={editPath}>{t("qa.edit")}</Link>
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
              <DropdownMenuItem destructive onSelect={onDelete}>
                {t("qa.delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </TableCell>
    </TableRow>
  )
}
