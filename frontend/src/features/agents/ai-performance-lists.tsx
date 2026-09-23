/** AI 表现报表的分页列表：按渠道或咨询分类拆分，以及待补知识清单。 */
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  AIPerformanceDimension,
  listAIKnowledgeGaps,
  listAIPerformanceBreakdowns,
} from "@/api"
import { ResourceListLayout } from "@/components/resource-list"
import { ResourceTable } from "@/components/resource-table"
import { handoffReasonKey } from "@/features/inbox/agent-process"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

import { useAIPerformanceFormat } from "./ai-performance-format"

/** 报表列表共用的统计范围与分页。 */
type ReportListProps = {
  days: number
  channelId: string
  page: number
  onPageChange: (page: number) => void
}

const pageSize = 50

/** 按渠道或咨询分类列出已结束会话数、AI 独立解决数与解决率。 */
export function AIPerformanceBreakdownList({
  dimension,
  days,
  channelId,
  page,
  onPageChange,
}: ReportListProps & { dimension: AIPerformanceDimension }) {
  const { t } = useTranslation("agents")
  const { count, rate } = useAIPerformanceFormat()
  const parameters = { days, channelId, dimension, page, pageSize }
  const list = useResource(
    resourceKeys.aiPerformanceBreakdowns(parameters),
    () => listAIPerformanceBreakdowns(parameters),
    { keepPreviousData: true },
  )

  return (
    <ResourceListLayout
      resources={list}
      errorMessage={t("performance.loadError")}
      page={list.data?.page}
      onPageChange={onPageChange}
    >
      <ResourceTable
        columns={[
          {
            key: "name",
            header: t("performance.columns.name"),
            cellClassName: "max-w-64 truncate",
            cell: (row) => row.name || t("performance.uncategorized"),
          },
          {
            key: "closed",
            header: t("performance.columns.closed"),
            className: "w-28 text-right tabular-nums",
            cell: (row) => count(row.closed),
          },
          {
            key: "aiResolved",
            header: t("performance.columns.aiResolved"),
            className: "w-32 text-right tabular-nums",
            cell: (row) => count(row.aiResolved),
          },
          {
            key: "rate",
            header: t("performance.columns.rate"),
            className: "w-24 text-right tabular-nums",
            cell: (row) => rate(row.aiResolved, row.closed),
          },
        ]}
        rows={list.data?.rows ?? []}
        rowKey={(row) => row.id || "uncategorized"}
        empty={t("performance.noSessions")}
      />
    </ResourceListLayout>
  )
}

/** 列出因知识不足或缺少依据的转人工，点击在收件箱定位客户提问。 */
export function AIKnowledgeGapList({ days, channelId, page, onPageChange }: ReportListProps) {
  const { t } = useTranslation(["agents", "inbox"])
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const parameters = { days, channelId, page, pageSize }
  const list = useResource(
    resourceKeys.aiKnowledgeGaps(parameters),
    () => listAIKnowledgeGaps(parameters),
    { keepPreviousData: true },
  )

  return (
    <ResourceListLayout
      resources={list}
      errorMessage={t("performance.loadError")}
      page={list.data?.page}
      onPageChange={onPageChange}
    >
      <ResourceTable
        hideHeader
        columns={[
          {
            key: "question",
            header: t("performance.gapQuestion"),
            cellClassName: "w-full max-w-0",
            cell: (gap) => (
              <div className="min-w-0">
                <p className="truncate">{gap.question || t("performance.noQuestion")}</p>
                <p className="truncate text-xs text-muted-foreground">
                  {[
                    gap.categoryName || t("performance.uncategorized"),
                    t(`inbox:${handoffReasonKey(gap.reason)}`),
                  ].join(" · ")}
                </p>
              </div>
            ),
          },
          {
            key: "time",
            header: t("performance.gapTime"),
            cellClassName: "whitespace-nowrap text-muted-foreground",
            cell: (gap) => t("performance.handedOffAt", { time: formatDateTime(gap.occurredAt) }),
          },
        ]}
        rows={list.data?.gaps ?? []}
        rowKey={(gap) => gap.eventId}
        empty={t("performance.noGaps")}
        onRowActivate={(gap) => {
          // 在收件箱打开该会话并定位到客户提问。
          const params = new URLSearchParams({ conversation: gap.conversationId })
          if (gap.messageId) params.set("message", gap.messageId)
          navigate(`/inbox?${params.toString()}`)
        }}
      />
    </ResourceListLayout>
  )
}
