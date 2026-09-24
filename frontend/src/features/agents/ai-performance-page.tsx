/** AI 表现报表页：概览、待补知识、按渠道与按咨询分类四个与地址同步的页签，共用渠道筛选；报表页签按结束时间筛选，待补知识按处理状态筛选，处理中的条目编号保存在地址中。 */
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import {
  AIPerformanceDimension,
  getAIPerformanceReport,
  KnowledgeGapStatus,
  listInboxChannels,
  type KnowledgeGapStatusId,
} from "@/api"
import { ListToolbar, ListToolbarFilter } from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

import { AIKnowledgeGapList, AIPerformanceBreakdownList } from "./ai-performance-lists"
import { AIPerformanceOverview } from "./ai-performance-overview"

/** 可选的统计天数，第一个为默认值。 */
const periodOptions = [30, 7, 90] as const

/** 页签，第一个为默认值。 */
const reportTabs = ["overview", "knowledgeGaps", "channels", "categories"] as const

type ReportTab = (typeof reportTabs)[number]

/** 待补知识的处理状态筛选，第一个为默认值。 */
const gapStatuses: KnowledgeGapStatusId[] = [
  KnowledgeGapStatus.KnowledgeGapStatusPending,
  KnowledgeGapStatus.KnowledgeGapStatusAccepted,
  KnowledgeGapStatus.KnowledgeGapStatusDismissed,
]

/** 地址参数的默认值，等于默认值时从地址中移除。 */
const parameterDefaults: Record<string, string> = {
  tab: reportTabs[0],
  days: String(periodOptions[0]),
  channel: "",
  status: gapStatuses[0],
  gap: "",
  page: "1",
}

/** 显示 AI 客服表现报表，页签、筛选和页码保存在地址中。 */
export function AIPerformancePage() {
  const { t } = useTranslation("agents")
  const [searchParams, setSearchParams] = useSearchParams()
  const days =
    periodOptions.find((option) => String(option) === searchParams.get("days")) ??
    periodOptions[0]
  const channelId = searchParams.get("channel") ?? ""
  // 选定渠道时不显示按渠道拆分。
  const tabs = reportTabs.filter((value) => !(channelId && value === "channels"))
  const tab = tabs.find((value) => value === searchParams.get("tab")) ?? tabs[0]
  const page = Math.max(1, Number(searchParams.get("page")) || 1)
  const status = gapStatuses.find((value) => value === searchParams.get("status")) ?? gapStatuses[0]
  const gapId = searchParams.get("gap") ?? ""

  const channels = useResource(resourceKeys.inboxChannels(), () => listInboxChannels(), {
    staleTime: 0,
  })
  const report = useResource(
    resourceKeys.aiPerformanceReport({ days, channelId }),
    () => getAIPerformanceReport({ days, channelId }),
    { keepPreviousData: true, enabled: tab === "overview" },
  )

  /** 更新地址参数；未给出页码时回到第一页，筛选变化时关闭处理中的条目。 */
  function setParameters(changes: {
    tab?: ReportTab
    days?: string
    channel?: string
    status?: string
    gap?: string
    page?: number
  }) {
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (changes.page === undefined) next.delete("page")
        if (changes.gap === undefined) next.delete("gap")
        for (const [name, value] of Object.entries(changes)) {
          const text = String(value)
          if (text === parameterDefaults[name]) next.delete(name)
          else next.set(name, text)
        }
        return next
      },
      { replace: true },
    )
  }

  const listProps = {
    days,
    channelId,
    page,
    onPageChange: (number: number) => setParameters({ page: number }),
  }

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("performance.title")} description={t("performance.description")} />

      <div className="cervi-page-gutter shrink-0">
        <Tabs value={tab} onValueChange={(value) => setParameters({ tab: value as ReportTab })}>
          <TabsList>
            {tabs.map((value) => (
              <TabsTrigger key={value} value={value}>
                {t(`performance.tabs.${value}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>

      <ListToolbar>
        {/* 待补知识是待办队列，不按结束时间筛选。 */}
        {tab === "knowledgeGaps" ? null : (
          <ListToolbarFilter
            label={t("performance.period")}
            value={String(days)}
            options={periodOptions.map((option) => ({
              value: String(option),
              label: t("performance.periodDays", { count: option }),
            }))}
            onValueChange={(value) => setParameters({ days: value })}
          />
        )}
        <ListToolbarFilter
          label={t("performance.channel")}
          allLabel={t("performance.allChannels")}
          value={channelId}
          options={(channels.data ?? []).map((channel) => ({
            value: channel.id,
            label: channel.name,
          }))}
          onValueChange={(value) => setParameters({ channel: value })}
        />
        {tab === "knowledgeGaps" ? (
          <ListToolbarFilter
            label={t("performance.gapStatus")}
            value={status}
            options={gapStatuses.map((value) => ({
              value,
              label: t(`performance.gapStatuses.${value}`),
            }))}
            onValueChange={(value) => setParameters({ status: value })}
          />
        ) : null}
      </ListToolbar>

      {tab === "overview" ? (
        <PageContent>
          <ResourceContent resources={report} errorMessage={t("performance.loadError")}>
            {report.data ? (
              <AIPerformanceOverview
                report={report.data}
                onOpenKnowledgeGaps={() =>
                  setParameters({ tab: "knowledgeGaps", status: gapStatuses[0] })
                }
              />
            ) : null}
          </ResourceContent>
        </PageContent>
      ) : tab === "knowledgeGaps" ? (
        <AIKnowledgeGapList
          channelId={channelId}
          page={page}
          onPageChange={listProps.onPageChange}
          status={status}
          gapId={gapId}
          onGapChange={(value) => setParameters({ gap: value, page })}
        />
      ) : (
        <AIPerformanceBreakdownList
          key={tab}
          dimension={
            tab === "channels"
              ? AIPerformanceDimension.AIPerformanceDimensionChannel
              : AIPerformanceDimension.AIPerformanceDimensionCategory
          }
          {...listProps}
        />
      )}
    </section>
  )
}
