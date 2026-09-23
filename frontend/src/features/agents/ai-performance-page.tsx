/** AI 表现报表页：共用的结束时间与渠道筛选，概览、待补知识、按渠道与按咨询分类四个与地址同步的页签。 */
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import { AIPerformanceDimension, getAIPerformanceReport, listInboxChannels } from "@/api"
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

/** 地址参数的默认值，等于默认值时从地址中移除。 */
const parameterDefaults: Record<string, string> = {
  tab: reportTabs[0],
  days: String(periodOptions[0]),
  channel: "",
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

  const channels = useResource(resourceKeys.inboxChannels(), () => listInboxChannels(), {
    staleTime: 0,
  })
  const report = useResource(
    resourceKeys.aiPerformanceReport({ days, channelId }),
    () => getAIPerformanceReport({ days, channelId }),
    { keepPreviousData: true, enabled: tab === "overview" },
  )

  /** 更新地址参数；未给出页码时回到第一页。 */
  function setParameters(changes: { tab?: ReportTab; days?: string; channel?: string; page?: number }) {
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (changes.page === undefined) next.delete("page")
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
        <ListToolbarFilter
          label={t("performance.period")}
          value={String(days)}
          options={periodOptions.map((option) => ({
            value: String(option),
            label: t("performance.periodDays", { count: option }),
          }))}
          onValueChange={(value) => setParameters({ days: value })}
        />
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
      </ListToolbar>

      {tab === "overview" ? (
        <PageContent>
          <ResourceContent resources={report} errorMessage={t("performance.loadError")}>
            {report.data ? (
              <AIPerformanceOverview
                report={report.data}
                onOpenKnowledgeGaps={() => setParameters({ tab: "knowledgeGaps" })}
              />
            ) : null}
          </ResourceContent>
        </PageContent>
      ) : tab === "knowledgeGaps" ? (
        <AIKnowledgeGapList {...listProps} />
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
