/** AI 员工独立创建页和详情页：详情页分概览、待补知识、服务记录、基本资料与执行五个与地址同步的页签。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router"

import {
  getAIPerformanceReport,
  getAgent,
  isNotFoundApiError,
  listTeams,
  type AgentData,
} from "@/api"
import { ListToolbar, ListToolbarFilter } from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageBackButton } from "@/components/page-back-button"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { agentReturnPath } from "@/features/agents/agent-navigation"
import { AgentForm } from "@/features/agents/agent-form"
import { AgentProfileForm } from "@/features/agents/agent-profile-form"
import { AgentExecutionForm } from "@/features/agents/agent-execution-form"
import { AgentServiceRecords } from "@/features/agents/agent-service-records"
import { aiIssueTypes, gapStatuses } from "@/features/agents/ai-performance-format"
import { AIKnowledgeGapList, AIPerformanceIssueList } from "@/features/agents/ai-performance-lists"
import { periodOptions } from "@/features/agents/report-format"
import { AIPerformanceOverview } from "@/features/agents/ai-performance-overview"
import { useContactInvalidator } from "@/hooks/use-contact-invalidator"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

/** 详情页签，第一个为默认值。 */
const detailTabs = ["overview", "knowledgeGaps", "issues", "records", "basic", "execution"] as const

type DetailTab = (typeof detailTabs)[number]

/** 加载 AI 员工详情；新建时只显示创建表单。 */
export function AgentFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation(["agents", "common"])
  const { agentId = "" } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const location = useLocation()
  const detail = useResource(
    resourceKeys.agent(agentId),
    () => getAgent(agentId),
    { enabled: mode === "edit" },
  )
  const agent = detail.data
  const tab = detailTabs.find((value) => value === searchParams.get("tab")) ?? detailTabs[0]
  const returnTo = agentReturnPath(location.pathname, location.search)
  const teamId = searchParams.get("teamId")

  // 缺省或无效页签统一写回地址，刷新时恢复同一页签。
  useEffect(() => {
    if (mode !== "edit" || searchParams.get("tab") === tab) return
    const next = new URLSearchParams(searchParams)
    next.set("tab", tab)
    setSearchParams(next, { replace: true })
  }, [mode, searchParams, setSearchParams, tab])

  // 员工不存在时返回来源列表。
  useEffect(() => {
    if (mode !== "edit" || !isNotFoundApiError(detail.error)) return
    console.warn("AI 员工不存在", { agent_id: agentId })
    navigate(returnTo, { replace: true })
  }, [agentId, detail.error, mode, navigate, returnTo])

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={mode === "create" ? t("create") : (agent?.displayName ?? t("editTitle"))}
        description={t(mode === "create" ? "createDescription" : "editDescription")}
      >
        {mode === "edit" ? <PageBackButton to={returnTo} /> : null}
      </PageHeader>
      {mode === "create" ? (
        <PageContent variant="form">
          <AgentForm
            defaultTeamIds={teamId ? [teamId] : []}
            onCancel={() => navigate(returnTo)}
            onSaved={() => navigate(returnTo, { replace: true })}
          />
        </PageContent>
      ) : (
        <ResourceContent resources={detail} errorMessage={t("form.loadError")}>
          {agent ? <AgentDetailTabs key={agent.id} agent={agent} tab={tab} /> : null}
        </ResourceContent>
      )}
    </div>
  )
}

/** AI 员工详情的页签与各页签内容；页签、统计天数、待补知识状态、问题类型与打开的条目保存在地址中，两个配置表单保持挂载以保留未保存的修改，团队只在基本资料中读取。 */
function AgentDetailTabs({ agent, tab }: { agent: AgentData; tab: DetailTab }) {
  const { t } = useTranslation("agents")
  const [searchParams, setSearchParams] = useSearchParams()
  const invalidateContact = useContactInvalidator()
  const invalidate = useResourceInvalidator()
  const teams = useResource(resourceKeys.teams({ pageSize: 100 }), () => listTeams({ pageSize: 100 }))
  const days =
    periodOptions.find((option) => String(option) === searchParams.get("days")) ??
    periodOptions[0]
  const status = gapStatuses.find((value) => value === searchParams.get("status")) ?? gapStatuses[0]
  const gapId = searchParams.get("gap") ?? ""
  const issue = aiIssueTypes.find((value) => value === searchParams.get("issue")) ?? aiIssueTypes[0]
  const sessionId = searchParams.get("session") ?? ""
  const filter = { channelId: "", agentId: agent.id, mine: false }
  const report = useResource(
    resourceKeys.aiPerformanceReport({ days, ...filter }),
    () => getAIPerformanceReport({ days, ...filter }),
    { keepPreviousData: true, enabled: tab === "overview" },
  )

  /** 更新地址参数，等于默认值的参数从地址中移除，未给出打开的条目时关闭该条目。 */
  function setParameters(changes: {
    tab?: DetailTab
    days?: string
    status?: string
    gap?: string
    issue?: string
    session?: string
  }) {
    const defaults: Record<string, string> = {
      days: String(periodOptions[0]),
      status: gapStatuses[0],
      gap: "",
      issue: aiIssueTypes[0],
      session: "",
    }
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (changes.gap === undefined) next.delete("gap")
        if (changes.session === undefined) next.delete("session")
        for (const [name, value] of Object.entries(changes)) {
          if (value === defaults[name]) next.delete(name)
          else next.set(name, value)
        }
        return next
      },
      { replace: true },
    )
  }

  return (
    <>
      <div className="app-page-gutter shrink-0">
        <Tabs value={tab} onValueChange={(value) => setParameters({ tab: value as DetailTab })}>
          <TabsList>
            {detailTabs.map((value) => (
              <TabsTrigger key={value} value={value}>
                {t(`detailTabs.${value}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>
      {tab === "overview" ? (
        <>
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
          </ListToolbar>
          <PageContent>
            <ResourceContent resources={report} errorMessage={t("performance.loadError")}>
              {report.data ? (
                <AIPerformanceOverview
                  report={report.data}
                  onOpenKnowledgeGaps={() => setParameters({ tab: "knowledgeGaps", status: gapStatuses[0] })}
                  onOpenIssues={(value) => setParameters({ tab: "issues", issue: value })}
                />
              ) : null}
            </ResourceContent>
          </PageContent>
        </>
      ) : null}
      {tab === "knowledgeGaps" ? (
        <>
          <ListToolbar>
            <ListToolbarFilter
              label={t("performance.gapStatus")}
              value={status}
              options={gapStatuses.map((value) => ({
                value,
                label: t(`performance.gapStatuses.${value}`),
              }))}
              onValueChange={(value) => setParameters({ status: value })}
            />
          </ListToolbar>
          <AIKnowledgeGapList
            filter={filter}
            status={status}
            gapId={gapId}
            onGapChange={(value) => setParameters({ gap: value })}
          />
        </>
      ) : null}
      {tab === "issues" ? (
        <>
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
              label={t("performance.issueType")}
              value={issue}
              options={aiIssueTypes.map((value) => ({
                value,
                label: t(`performance.issueTypes.${value}`),
              }))}
              onValueChange={(value) => setParameters({ issue: value })}
            />
          </ListToolbar>
          <AIPerformanceIssueList
            days={days}
            filter={filter}
            issue={issue}
            serviceSessionId={sessionId}
            onIssueOpen={(value) => setParameters({ session: value })}
          />
        </>
      ) : null}
      {tab === "records" ? <AgentServiceRecords agentId={agent.id} /> : null}
      <PageContent
        variant="form"
        className={cn(tab !== "basic" && tab !== "execution" && "hidden")}
      >
        <div className={cn(tab !== "basic" && "hidden")}>
          <ResourceContent resources={teams} errorMessage={t("form.loadError")}>
            <AgentProfileForm
              agent={agent}
              teams={teams.data?.teams ?? []}
              onSaved={() => {
                void invalidateContact("agent", agent.id)
                // 负责人变化影响「我负责的」范围。
                void invalidate(resourceKeys.knowledgeGaps())
                void invalidate(resourceKeys.aiPerformanceReport())
                void invalidate(resourceKeys.aiPerformanceBreakdowns())
                void invalidate(resourceKeys.aiPerformanceIssues())
              }}
            />
          </ResourceContent>
        </div>
        <div className={cn(tab !== "execution" && "hidden")}>
          <AgentExecutionForm
            agent={agent}
            onSaved={() => {
              void invalidateContact("agent", agent.id)
            }}
          />
        </div>
      </PageContent>
    </>
  )
}
