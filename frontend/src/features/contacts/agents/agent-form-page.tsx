/** AI 员工独立创建页和分组配置页。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router"

import { getAgent, isNotFoundApiError, listRoles, listTeams } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { agentReturnPath } from "@/features/contacts/agents/agent-navigation"
import { AgentForm } from "@/features/contacts/agents/agent-form"
import { AgentProfileForm } from "@/features/contacts/agents/agent-profile-form"
import { AgentExecutionForm } from "@/features/contacts/agents/agent-execution-form"
import { useContactInvalidator } from "@/features/contacts/use-contact-invalidator"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 加载 AI 员工配置并维护独立保存的两个页签。 */
export function AgentFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation(["contacts", "common"])
  const { agentId = "" } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const location = useLocation()
  const invalidateContact = useContactInvalidator()
  const roles = useResource(resourceKeys.roles(), () => listRoles())
  const teams = useResource(
    resourceKeys.teams({ pageSize: 100 }),
    () => listTeams({ pageSize: 100 }),
    { enabled: mode === "edit" },
  )
  const detail = useResource(
    resourceKeys.agent(agentId),
    () => getAgent(agentId),
    { enabled: mode === "edit" },
  )
  const agent = detail.data
  const tab = searchParams.get("tab") === "execution" ? "execution" : "basic"
  const returnTo = agentReturnPath(location.pathname, location.search)
  const teamId = searchParams.get("teamId")
  const loading =
    roles.loading || (mode === "edit" && (teams.loading || detail.loading))
  const error =
    (!roles.data ? roles.error : null) ??
    (mode === "edit" && !teams.data ? teams.error : null) ??
    (mode === "edit" && !agent ? detail.error : null)

  // 缺省或无效页签统一写回地址，刷新时恢复同一分组。
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
        title={
          mode === "create"
            ? t("agents.create")
            : agent
              ? t("agents.edit", { name: agent.displayName })
              : t("agents.editTitle")
        }
      />
      <PageContent>
        {loading ? (
          <LoadingIndicator className="min-h-48 justify-center">
            {t("common:status.loading")}
          </LoadingIndicator>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center gap-4">
            <p className="text-sm text-muted-foreground">
              {t("agents.form.loadError")}
            </p>
            <Button
              variant="outline"
              onClick={() => {
                if (roles.error) void roles.refresh()
                if (mode === "edit") {
                  if (teams.error) void teams.refresh()
                  if (detail.error) void detail.refresh()
                }
              }}
            >
              {t("common:actions.retry")}
            </Button>
          </div>
        ) : mode === "create" ? (
          <AgentForm
            roles={roles.data?.roles ?? []}
            defaultTeamIds={teamId ? [teamId] : []}
            onCancel={() => navigate(returnTo)}
            onSaved={(created) => {
              const next = new URLSearchParams({ tab: "basic", returnTo })
              navigate(`/contacts/ai-employees/${created.id}?${next}`, {
                replace: true,
              })
            }}
          />
        ) : agent ? (
          <Tabs
            key={agent.id}
            value={tab}
            className="max-w-2xl"
            onValueChange={(value) => {
              const next = new URLSearchParams(searchParams)
              next.set("tab", value)
              setSearchParams(next, { replace: true })
            }}
          >
            <TabsList>
              <TabsTrigger value="basic">{t("agents.basic")}</TabsTrigger>
              <TabsTrigger value="execution">
                {t("agents.execution.title")}
              </TabsTrigger>
            </TabsList>
            <TabsContent
              value="basic"
              forceMount
              className="mt-6 data-[state=inactive]:hidden"
            >
              <AgentProfileForm
                agent={agent}
                roles={roles.data?.roles ?? []}
                teams={teams.data?.teams ?? []}
                onSaved={() => {
                  void invalidateContact("agent", agent.id)
                }}
                onCancel={() => navigate(returnTo)}
              />
            </TabsContent>
            <TabsContent
              value="execution"
              forceMount
              className="mt-6 data-[state=inactive]:hidden"
            >
              <AgentExecutionForm
                agent={agent}
                onSaved={() => {
                  void invalidateContact("agent", agent.id)
                }}
                onCancel={() => navigate(returnTo)}
              />
            </TabsContent>
          </Tabs>
        ) : null}
      </PageContent>
    </div>
  )
}
