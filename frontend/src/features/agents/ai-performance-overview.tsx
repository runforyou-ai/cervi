/** AI 表现报表概览：指标卡、解决情况、结束方式与转人工原因分布。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { AgentHandoffReason, type AIPerformanceReportData } from "@/api"
import { handoffReasonKey } from "@/lib/handoff-reason-labels"
import { cn } from "@/lib/utils"

import { useAIPerformanceFormat } from "./ai-performance-format"

/** 模型调用 handoff_to_human 时可给出的业务原因，其余为系统原因。 */
const businessReasons: readonly AgentHandoffReason[] = [
  AgentHandoffReason.AgentHandoffReasonKnowledgeGap,
  AgentHandoffReason.AgentHandoffReasonCustomerRequested,
  AgentHandoffReason.AgentHandoffReasonNeedsHumanJudgment,
  AgentHandoffReason.AgentHandoffReasonComplaint,
]

/** 显示概览指标，点击待补知识指标卡进入待补知识页签。 */
export function AIPerformanceOverview({
  report,
  onOpenKnowledgeGaps,
}: {
  report: AIPerformanceReportData
  onOpenKnowledgeGaps: () => void
}) {
  const { t } = useTranslation(["agents", "inbox"])
  const { count, rate } = useAIPerformanceFormat()
  const { summary, handoffReasons } = report
  const handoffTotal = handoffReasons.reduce((sum, item) => sum + item.count, 0)

  return (
    <div className="space-y-8">
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatTile
          label={t("performance.aiResolvedRate")}
          value={rate(summary.aiResolved, summary.closed)}
          detail={t("performance.aiResolvedDetail", {
            resolved: count(summary.aiResolved),
            closed: count(summary.closed),
          })}
        />
        <StatTile
          label={t("performance.ratingRate")}
          value={rate(summary.ratedResolved, summary.rated)}
          detail={t("performance.ratingDetail", { formatted: count(summary.rated) })}
        />
        <StatTile
          label={t("performance.handoffRate")}
          value={rate(summary.handedOff, summary.closed)}
          detail={t("performance.handoffRateDetail", { formatted: count(summary.handedOff) })}
        />
        <StatTile
          label={t("performance.gaps")}
          value={count(report.knowledgeGapTotal)}
          detail={t("performance.gapsDetail")}
          onClick={onOpenKnowledgeGaps}
        />
      </div>

      <ReportSection title={t("performance.resolution")}>
        {summary.closed > 0 ? (
          <div className="grid gap-x-8 gap-y-5 md:grid-cols-2">
            {[
              {
                key: "ai",
                title: t("performance.aiOnly"),
                total: summary.aiOnly,
                resolved: summary.aiResolved,
                unresolved: summary.aiUnresolved,
              },
              {
                key: "human",
                title: t("performance.humanInvolved"),
                total: summary.closed - summary.aiOnly,
                resolved: summary.resolved - summary.aiResolved,
                unresolved: summary.unresolved - summary.aiUnresolved,
              },
            ].map((group) => {
              if (group.total === 0) return null
              return (
                <div key={group.key} className="space-y-2">
                  <p className="text-xs text-muted-foreground">
                    {t("performance.resolutionGroup", { name: group.title, formatted: count(group.total) })}
                  </p>
                  <MeterList
                    total={group.total}
                    rows={[
                      { key: "resolved", label: t("performance.resolved"), value: group.resolved },
                      { key: "unresolved", label: t("performance.unresolved"), value: group.unresolved },
                      {
                        key: "undetermined",
                        label: t("performance.undetermined"),
                        value: group.total - group.resolved - group.unresolved,
                      },
                    ]}
                    format={(value) => `${count(value)} · ${rate(value, group.total)}`}
                  />
                </div>
              )
            })}
          </div>
        ) : (
          <EmptyNote>{t("performance.noSessions")}</EmptyNote>
        )}
      </ReportSection>

      <div className="grid gap-8 md:grid-cols-2">
        <ReportSection title={t("performance.closeReasons")}>
          {summary.closed > 0 ? (
            <MeterList
              total={summary.closed}
              rows={[
                { key: "ai", label: t("performance.closeAIResolved"), value: summary.closeAiResolved },
                { key: "unresponsive", label: t("performance.closeCustomerUnresponsive"), value: summary.customerUnresponsive },
                { key: "manual", label: t("performance.closeManual"), value: summary.manual },
              ]}
              format={(value) => `${count(value)} · ${rate(value, summary.closed)}`}
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
                const rows = handoffReasons.filter(
                  (item) => businessReasons.includes(item.reason) === group.business,
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
                      format={(value) => `${count(value)} · ${rate(value, handoffTotal)}`}
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
    </div>
  )
}

/** 指标卡：名称、主数值与一行说明；给出 onClick 时整卡可点击。 */
function StatTile({
  label,
  value,
  detail,
  onClick,
}: {
  label: string
  value: string
  detail: string
  onClick?: () => void
}) {
  const content = (
    <>
      <p className="truncate text-sm text-muted-foreground">{label}</p>
      <p className="mt-1.5 text-2xl font-semibold tabular-nums">{value}</p>
      <p className="mt-1 truncate text-xs text-muted-foreground">{detail}</p>
    </>
  )
  const className = "rounded-lg border border-border/55 px-4 py-3.5 text-left"
  return onClick ? (
    <button
      type="button"
      className={cn(
        className,
        "transition-colors hover:bg-accent focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
      )}
      onClick={onClick}
    >
      {content}
    </button>
  ) : (
    <div className={className}>{content}</div>
  )
}

/** 概览分区：小标题与内容。 */
function ReportSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h3 className="text-sm font-medium">{title}</h3>
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
