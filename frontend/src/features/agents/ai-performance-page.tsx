/** AI 表现报表页：AI 独立解决率、结束方式、转人工原因、按渠道与分类拆分以及待补知识清单。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearchParams } from "react-router"

import {
  AgentHandoffReason,
  getAIPerformanceReport,
  listInboxChannels,
  type AIHandoffReasonCount,
  type AIKnowledgeGap,
  type AIPerformanceBreakdown,
} from "@/api"
import { ListToolbar, ListToolbarFilter } from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import { ResourceListFrame } from "@/components/resource-list"
import { ResourceTable } from "@/components/resource-table"
import { handoffReasonKey } from "@/features/inbox/agent-process"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 可选的统计天数，第一个为默认值。 */
const periodOptions = [30, 7, 90] as const

/** 模型调用 handoff_to_human 时可给出的业务原因，其余为系统原因。 */
const businessReasons: readonly AgentHandoffReason[] = [
  AgentHandoffReason.AgentHandoffReasonKnowledgeGap,
  AgentHandoffReason.AgentHandoffReasonCustomerRequested,
  AgentHandoffReason.AgentHandoffReasonNeedsHumanJudgment,
  AgentHandoffReason.AgentHandoffReasonComplaint,
]

/** 显示 AI 客服表现报表，筛选条件保存在地址中。 */
export function AIPerformancePage() {
  const { t, i18n } = useTranslation(["agents", "inbox"])
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [searchParams, setSearchParams] = useSearchParams()
  const days =
    periodOptions.find((option) => String(option) === searchParams.get("days")) ??
    periodOptions[0]
  const channelId = searchParams.get("channel") ?? ""

  const channels = useResource(resourceKeys.inboxChannels(), () => listInboxChannels(), {
    staleTime: 0,
  })
  const report = useResource(
    resourceKeys.aiPerformanceReport({ days, channelId }),
    () => getAIPerformanceReport({ days, channelId }),
    { keepPreviousData: true },
  )
  const data = report.data
  const percent = new Intl.NumberFormat(i18n.resolvedLanguage, {
    style: "percent",
    maximumFractionDigits: 1,
  })
  const count = new Intl.NumberFormat(i18n.resolvedLanguage)

  /** 返回占比文本，分母为 0 时显示占位符。 */
  function rate(part: number, total: number) {
    return total > 0 ? percent.format(part / total) : t("performance.empty")
  }

  /** 更新地址中的筛选参数，默认值从地址中移除。 */
  function setFilter(name: "days" | "channel", value: string) {
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (value && !(name === "days" && value === String(periodOptions[0]))) next.set(name, value)
        else next.delete(name)
        return next
      },
      { replace: true },
    )
  }

  const summary = data?.summary
  const handoffTotal = (data?.handoffReasons ?? []).reduce((sum, item) => sum + item.count, 0)
  const breakdownColumns = [
    {
      key: "name",
      header: t("performance.columns.name"),
      cellClassName: "max-w-64 truncate",
      cell: (row: AIPerformanceBreakdown) => row.name || t("performance.uncategorized"),
    },
    {
      key: "closed",
      header: t("performance.columns.closed"),
      className: "w-28 text-right tabular-nums",
      cell: (row: AIPerformanceBreakdown) => count.format(row.closed),
    },
    {
      key: "aiResolved",
      header: t("performance.columns.aiResolved"),
      className: "w-32 text-right tabular-nums",
      cell: (row: AIPerformanceBreakdown) => count.format(row.aiResolved),
    },
    {
      key: "rate",
      header: t("performance.columns.rate"),
      className: "w-24 text-right tabular-nums",
      cell: (row: AIPerformanceBreakdown) => rate(row.aiResolved, row.closed),
    },
  ]

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("performance.title")} description={t("performance.description")} />

      <ListToolbar>
        <ListToolbarFilter
          label={t("performance.period")}
          value={String(days)}
          options={periodOptions.map((option) => ({
            value: String(option),
            label: t("performance.periodDays", { count: option }),
          }))}
          onValueChange={(value) => setFilter("days", value)}
        />
        <ListToolbarFilter
          label={t("performance.channel")}
          allLabel={t("performance.allChannels")}
          value={channelId}
          options={(channels.data ?? []).map((channel) => ({
            value: channel.id,
            label: channel.name,
          }))}
          onValueChange={(value) => setFilter("channel", value)}
        />
      </ListToolbar>

      <PageContent className="space-y-8">
        <ResourceContent resources={report} errorMessage={t("performance.loadError")}>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <StatTile
              label={t("performance.aiResolvedRate")}
              value={rate(summary?.aiResolved ?? 0, summary?.closed ?? 0)}
              detail={t("performance.aiResolvedDetail", {
                resolved: count.format(summary?.aiResolved ?? 0),
                closed: count.format(summary?.closed ?? 0),
              })}
            />
            <StatTile
              label={t("performance.ratingRate")}
              value={rate(summary?.ratedResolved ?? 0, summary?.rated ?? 0)}
              detail={t("performance.ratingDetail", { formatted: count.format(summary?.rated ?? 0) })}
            />
            <StatTile
              label={t("performance.handoffRate")}
              value={rate(summary?.handedOff ?? 0, summary?.closed ?? 0)}
              detail={t("performance.handoffRateDetail", { formatted: count.format(summary?.handedOff ?? 0) })}
            />
            <StatTile
              label={t("performance.gaps")}
              value={count.format(data?.knowledgeGapTotal ?? 0)}
              detail={t("performance.gapsDetail")}
            />
          </div>

          <div className="grid gap-8 md:grid-cols-2">
            <ReportSection title={t("performance.closeReasons")}>
              {summary && summary.closed > 0 ? (
                <MeterList
                  total={summary.closed}
                  rows={[
                    { key: "ai", label: t("performance.closeAIResolved"), value: summary.closeAiResolved },
                    { key: "unresponsive", label: t("performance.closeCustomerUnresponsive"), value: summary.customerUnresponsive },
                    { key: "manual", label: t("performance.closeManual"), value: summary.manual },
                  ]}
                  format={(value) => `${count.format(value)} · ${rate(value, summary.closed)}`}
                />
              ) : (
                <EmptyNote>{t("performance.noSessions")}</EmptyNote>
              )}
            </ReportSection>

            <ReportSection title={t("performance.handoffReasons")}>
              {handoffTotal > 0 ? (
                <div className="space-y-5">
                  {[
                    { key: "business", title: t("performance.businessReasons"), business: true },
                    { key: "system", title: t("performance.systemReasons"), business: false },
                  ].map((group) => {
                    const rows = (data?.handoffReasons ?? []).filter(
                      (item: AIHandoffReasonCount) => businessReasons.includes(item.reason) === group.business,
                    )
                    if (rows.length === 0) return null
                    return (
                      <div key={group.key} className="space-y-2">
                        <p className="text-xs text-muted-foreground">{group.title}</p>
                        <MeterList
                          total={handoffTotal}
                          rows={rows.map((item) => ({
                            key: item.reason,
                            label: t(`inbox:${handoffReasonKey(item.reason)}`),
                            value: item.count,
                          }))}
                          format={(value) => `${count.format(value)} · ${rate(value, handoffTotal)}`}
                        />
                      </div>
                    )
                  })}
                </div>
              ) : (
                <EmptyNote>{t("performance.noHandoffs")}</EmptyNote>
              )}
            </ReportSection>
          </div>

          {channelId ? null : (
            <ReportSection title={t("performance.byChannel")}>
              <ResourceListFrame>
                <ResourceTable
                  columns={breakdownColumns}
                  rows={data?.channels ?? []}
                  rowKey={(row) => row.id}
                  empty={t("performance.noSessions")}
                />
              </ResourceListFrame>
            </ReportSection>
          )}

          <ReportSection title={t("performance.byCategory")}>
            <ResourceListFrame>
              <ResourceTable
                columns={breakdownColumns}
                rows={data?.categories ?? []}
                rowKey={(row) => row.id || "uncategorized"}
                empty={t("performance.noSessions")}
              />
            </ResourceListFrame>
          </ReportSection>

          <ReportSection
            title={t("performance.gapList")}
            meta={
              data && data.knowledgeGapTotal > data.knowledgeGaps.length
                ? t("performance.gapListRecent", {
                    shown: data.knowledgeGaps.length,
                    total: data.knowledgeGapTotal,
                  })
                : undefined
            }
          >
            <ResourceListFrame>
              <ResourceTable
                hideHeader
                columns={[
                  {
                    key: "question",
                    header: t("performance.gapList"),
                    cellClassName: "max-w-0 w-full",
                    cell: (gap: AIKnowledgeGap) => (
                      <div className="min-w-0">
                        <p className="truncate">
                          {gap.question || t("performance.noQuestion")}
                        </p>
                        <p className="truncate text-xs text-muted-foreground">
                          {[gap.categoryName || t("performance.uncategorized"), t(`inbox:${handoffReasonKey(gap.reason)}`)].join(" · ")}
                        </p>
                      </div>
                    ),
                  },
                  {
                    key: "time",
                    header: t("performance.gapList"),
                    cellClassName: "whitespace-nowrap text-muted-foreground",
                    cell: (gap: AIKnowledgeGap) =>
                      t("performance.handedOffAt", { time: formatDateTime(gap.occurredAt) }),
                  },
                ]}
                rows={data?.knowledgeGaps ?? []}
                rowKey={(gap) => gap.eventId}
                empty={t("performance.noGaps")}
                onRowActivate={(gap) => {
                  // 在收件箱打开该会话并定位到客户提问。
                  const params = new URLSearchParams({ conversation: gap.conversationId })
                  if (gap.messageId) params.set("message", gap.messageId)
                  navigate(`/inbox?${params.toString()}`)
                }}
              />
            </ResourceListFrame>
          </ReportSection>
        </ResourceContent>
      </PageContent>
    </section>
  )
}

/** 指标卡：名称、主数值与一行说明。 */
function StatTile({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="rounded-lg border border-border/55 px-4 py-3.5">
      <p className="truncate text-sm text-muted-foreground">{label}</p>
      <p className="mt-1.5 text-2xl font-semibold tabular-nums">{value}</p>
      <p className="mt-1 truncate text-xs text-muted-foreground">{detail}</p>
    </div>
  )
}

/** 报表分区：小标题、可选的右侧补充信息与内容。 */
function ReportSection({ title, meta, children }: { title: string; meta?: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="text-sm font-medium">{title}</h3>
        {meta ? <p className="text-xs text-muted-foreground">{meta}</p> : null}
      </div>
      {children}
    </section>
  )
}

/** 按占总数比例绘制的横向条形列表，数值以文字写在行尾。 */
function MeterList({
  rows,
  total,
  format,
}: {
  rows: { key: string; label: string; value: number }[]
  total: number
  format: (value: number) => string
}) {
  return (
    <ul className="space-y-2.5">
      {rows.map((row) => (
        <li key={row.key} className="space-y-1" title={`${row.label} ${format(row.value)}`}>
          <div className="flex items-baseline justify-between gap-3 text-sm">
            <span className="truncate">{row.label}</span>
            <span className="shrink-0 text-muted-foreground tabular-nums">{format(row.value)}</span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-primary"
              style={{ width: `${total > 0 ? (row.value / total) * 100 : 0}%` }}
            />
          </div>
        </li>
      ))}
    </ul>
  )
}

/** 分区内没有数据时的说明。 */
function EmptyNote({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-sm text-muted-foreground">{children}</p>
}
