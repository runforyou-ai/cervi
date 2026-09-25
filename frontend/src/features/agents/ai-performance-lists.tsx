/** AI 表现报表的分页列表：按渠道或咨询分类拆分，以及待补知识清单。 */
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  AIPerformanceDimension,
  dismissKnowledgeGap,
  isApiError,
  KnowledgeGapSource,
  KnowledgeGapStatus,
  listAIPerformanceBreakdowns,
  listKnowledgeGaps,
  type KnowledgeGapStatusId,
} from "@/api"
import { ResourceListLayout } from "@/components/resource-list"
import { ResourceTable } from "@/components/resource-table"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

import { AIKnowledgeGapSheet } from "./ai-knowledge-gap-sheet"
import { useAIPerformanceFormat } from "./ai-performance-format"

/** 报表列表共用的统计范围与分页。 */
type ReportListProps = {
  days: number
  channelId: string
  page: number
  onPageChange: (page: number) => void
}

const pageSize = 50

/** 按渠道或咨询分类列出已结束会话数，以及已解决与 AI 独立解决的数量和占比。 */
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
            key: "resolved",
            header: t("performance.columns.resolved"),
            className: "w-36 text-right tabular-nums",
            cell: (row) => `${count(row.resolved)} · ${rate(row.resolved, row.closed)}`,
          },
          {
            key: "aiResolved",
            header: t("performance.columns.aiResolved"),
            className: "w-36 text-right tabular-nums",
            cell: (row) => `${count(row.aiResolved)} · ${rate(row.aiResolved, row.closed)}`,
          },
        ]}
        rows={list.data?.rows ?? []}
        rowKey={(row) => row.id || "uncategorized"}
        empty={t("performance.noSessions")}
      />
    </ResourceListLayout>
  )
}

/** 列出指定处理状态的待补知识，点击行在侧栏中处理，处理完一条自动打开清单中的下一条。 */
export function AIKnowledgeGapList({
  channelId,
  status,
  page,
  onPageChange,
  gapId,
  onGapChange,
}: Omit<ReportListProps, "days"> & {
  status: KnowledgeGapStatusId
  gapId: string
  onGapChange: (gapId: string) => void
}) {
  const { t } = useTranslation("agents")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const { formatDateTime } = useDateTime()
  const parameters = { channelId, status, page, pageSize }
  const list = useResource(
    resourceKeys.knowledgeGaps(parameters),
    () => listKnowledgeGaps(parameters),
    { keepPreviousData: true },
  )
  const rows = list.data?.gaps ?? []
  const pending = status === KnowledgeGapStatus.KnowledgeGapStatusPending

  /** 忽略清单中的一条待补知识。 */
  async function dismiss(id: string) {
    try {
      await dismissKnowledgeGap(id)
      await Promise.all([
        invalidate(resourceKeys.knowledgeGaps()),
        invalidate(resourceKeys.knowledgeGap(id)),
        invalidate(resourceKeys.aiPerformanceReport()),
      ])
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("忽略待补知识失败", error)
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("performance.gapSheet.dismissError"))
    }
  }

  return (
    <>
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
                      t(`performance.gapSources.${gap.source}`),
                      pending && gap.hasDraft ? t("performance.hasDraft") : "",
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </p>
                </div>
              ),
            },
            {
              key: "time",
              header: t("performance.gapTime"),
              cellClassName: "w-px whitespace-nowrap text-right text-muted-foreground",
              cell: (gap) => t(`performance.gapTimes.${gap.source}`, { time: formatDateTime(gap.occurredAt) }),
            },
          ]}
          rows={rows}
          rowKey={(gap) => gap.id}
          empty={t(`performance.noGaps.${status}`)}
          onRowActivate={(gap) => onGapChange(gap.id)}
          rowActions={(gap) => [
            {
              key: "conversation",
              label: t("performance.viewConversation"),
              onSelect: () => {
                // 在收件箱打开该会话；客户提问已确定时定位到提问，复核来源的提问由起草确定。
                const params = new URLSearchParams({ conversation: gap.conversationId })
                const review =
                  gap.source === KnowledgeGapSource.KnowledgeGapSourceRatedUnresolved ||
                  gap.source === KnowledgeGapSource.KnowledgeGapSourcePossiblyWrong
                if (gap.questionMessageId && (!review || gap.hasDraft)) params.set("message", gap.questionMessageId)
                navigate(`/inbox?${params.toString()}`)
              },
            },
            // 只有待处理的条目可以忽略。
            ...(gap.status === KnowledgeGapStatus.KnowledgeGapStatusPending
              ? [
                  {
                    key: "dismiss",
                    label: t("performance.dismissGap"),
                    separatorBefore: true,
                    onSelect: () => void dismiss(gap.id),
                  },
                ]
              : []),
          ]}
        />
      </ResourceListLayout>
      <AIKnowledgeGapSheet
        gapId={gapId}
        onClose={() => onGapChange("")}
        onHandled={() => {
          // 待处理清单打开当前条目之后的下一条，其余清单处理后关闭侧栏。
          const index = rows.findIndex((gap) => gap.id === gapId)
          onGapChange(pending && index >= 0 ? (rows[index + 1]?.id ?? "") : "")
        }}
      />
    </>
  )
}
