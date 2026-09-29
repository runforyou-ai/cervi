/** 团队表现报表页：概览、客服、问题会话、按渠道与按咨询分类五个与地址同步的页签，共用结束时间、渠道与队列筛选；问题会话按问题类型筛选，打开的条目编号保存在地址中。 */
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import { getTeamPerformanceReport, listInboxChannels, listTeams, ServiceReportDimension } from "@/api"
import { ListToolbar, ListToolbarFilter } from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

import { periodOptions } from "./report-format"
import {
  TeamPerformanceBreakdownList,
  TeamPerformanceIssueList,
  TeamPerformanceMemberList,
  teamIssueTypes,
  type TeamReportFilter,
} from "./team-performance-lists"
import { TeamPerformanceOverview } from "./team-performance-overview"

/** 页签，第一个为默认值。 */
const reportTabs = ["overview", "members", "issues", "channels", "categories"] as const

type ReportTab = (typeof reportTabs)[number]

/** 团队筛选中表示公共队列的取值。 */
const publicQueueFilter = "public"

/** 地址参数的默认值，等于默认值时从地址中移除。 */
const parameterDefaults: Record<string, string> = {
  tab: reportTabs[0],
  days: String(periodOptions[0]),
  channel: "",
  team: "",
  issue: teamIssueTypes[0],
  session: "",
}

/** 显示真人客服表现报表，页签和筛选保存在地址中。 */
export function TeamPerformancePage() {
  const { t } = useTranslation("agents")
  const [searchParams, setSearchParams] = useSearchParams()
  const team = searchParams.get("team") ?? ""
  const filter: TeamReportFilter = {
    days:
      periodOptions.find((option) => String(option) === searchParams.get("days")) ?? periodOptions[0],
    channelId: searchParams.get("channel") ?? "",
    teamId: team === publicQueueFilter ? "" : team,
    publicQueue: team === publicQueueFilter,
  }
  // 选定渠道时不显示按渠道拆分。
  const tabs = reportTabs.filter((value) => !(filter.channelId && value === "channels"))
  const tab = tabs.find((value) => value === searchParams.get("tab")) ?? tabs[0]
  const issue = teamIssueTypes.find((value) => value === searchParams.get("issue")) ?? teamIssueTypes[0]
  const sessionId = searchParams.get("session") ?? ""

  const channels = useResource(resourceKeys.inboxChannels(), () => listInboxChannels(), {
    staleTime: 0,
  })
  const teams = useResource(resourceKeys.teams({ pageSize: 100 }), () => listTeams({ pageSize: 100 }))
  const report = useResource(
    resourceKeys.teamPerformanceReport(filter),
    () => getTeamPerformanceReport(filter),
    { keepPreviousData: true, enabled: tab === "overview" },
  )

  /** 更新地址参数，未给出打开的条目时关闭该条目。 */
  function setParameters(changes: {
    tab?: ReportTab
    days?: string
    channel?: string
    team?: string
    issue?: string
    session?: string
  }) {
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (changes.session === undefined) next.delete("session")
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

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("teamPerformance.title")} description={t("teamPerformance.description")} />

      <div className="app-page-gutter shrink-0">
        <Tabs value={tab} onValueChange={(value) => setParameters({ tab: value as ReportTab })}>
          <TabsList>
            {tabs.map((value) => (
              <TabsTrigger key={value} value={value}>
                {t(`teamPerformance.tabs.${value}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>

      <ListToolbar>
        <ListToolbarFilter
          label={t("performance.period")}
          value={String(filter.days)}
          options={periodOptions.map((option) => ({
            value: String(option),
            label: t("performance.periodDays", { count: option }),
          }))}
          onValueChange={(value) => setParameters({ days: value })}
        />
        <ListToolbarFilter
          label={t("performance.channel")}
          allLabel={t("performance.allChannels")}
          value={filter.channelId}
          options={(channels.data ?? []).map((channel) => ({
            value: channel.id,
            label: channel.name,
          }))}
          onValueChange={(value) => setParameters({ channel: value })}
        />
        <ListToolbarFilter
          label={t("teamPerformance.team")}
          allLabel={t("teamPerformance.allTeams")}
          value={team}
          options={[
            { value: publicQueueFilter, label: t("teamPerformance.publicQueue") },
            ...(teams.data?.teams ?? []).map((item) => ({ value: item.id, label: item.name })),
          ]}
          onValueChange={(value) => setParameters({ team: value })}
        />
        {tab === "issues" ? (
          <ListToolbarFilter
            label={t("performance.issueType")}
            value={issue}
            options={teamIssueTypes.map((value) => ({
              value,
              label: t(`performance.issueTypes.${value}`),
            }))}
            onValueChange={(value) => setParameters({ issue: value })}
          />
        ) : null}
      </ListToolbar>

      {tab === "overview" ? (
        <PageContent>
          <ResourceContent resources={report} errorMessage={t("teamPerformance.loadError")}>
            {report.data ? (
              <TeamPerformanceOverview
                report={report.data}
                onOpenIssues={(value) => setParameters({ tab: "issues", issue: value })}
              />
            ) : null}
          </ResourceContent>
        </PageContent>
      ) : tab === "members" ? (
        <TeamPerformanceMemberList filter={filter} />
      ) : tab === "issues" ? (
        <TeamPerformanceIssueList
          filter={filter}
          issue={issue}
          serviceSessionId={sessionId}
          onIssueOpen={(value) => setParameters({ session: value })}
        />
      ) : (
        <TeamPerformanceBreakdownList
          key={tab}
          dimension={
            tab === "channels"
              ? ServiceReportDimension.ServiceReportDimensionChannel
              : ServiceReportDimension.ServiceReportDimensionCategory
          }
          filter={filter}
        />
      )}
    </section>
  )
}
