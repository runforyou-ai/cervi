/** 移动端团队列表与团队成员的只读浏览。 */
import { useEffect, useState } from "react"
import { ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useParams, useSearchParams } from "react-router"

import { listTeamMembers, listTeams, OrganizationIdentityType } from "@/api"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { MobilePagedList } from "@/apps/mobile/mobile-paged-list"
import { ProfileAvatar } from "@/components/profile-avatar"
import { WorkStatusBadge } from "@/components/work-status"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { useResource } from "@/hooks/use-resource"

const teamListQuery = { query: "", page: 1, pageSize: 50 }

/** 展示企业团队及其成员人数，点击进入成员名单。 */
export function MobileTeamsPage() {
  const { t } = useTranslation("mobile")
  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t("contacts.teams")} backTo="/contacts" />
      <MobilePagedList
        storageKey="teams"
        labels={{
          loadError: t("teams.loadError"),
          loadMoreError: t("contacts.loadMoreError"),
          empty: t("teams.empty"),
          allLoaded: t("teams.allLoaded"),
        }}
        source={(page) => {
          const query = { ...teamListQuery, page }
          return {
            key: resourceKeys.teams(query),
            load: (signal) => listTeams(query, signal),
          }
        }}
        select={(data) => ({ items: data.teams, page: data.page })}
      >
        {(teams) => (
          <ul className="divide-y border-b">
            {teams.map((team) => (
              <li key={team.id}>
                <Link
                  to={`/contacts/teams/${team.id}`}
                  state={{ mobileBack: true }}
                  className="flex min-h-18 items-center gap-3 px-4 py-3 outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[15px] font-medium">
                      {team.name}
                    </span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {team.description ||
                        t("teams.memberCount", { count: team.memberCount })}
                    </span>
                  </span>
                  <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </MobilePagedList>
    </section>
  )
}

/** 防抖同步姓名搜索，展示团队内的真人成员和 AI 员工。 */
export function MobileTeamMembersPage() {
  const { t } = useTranslation(["mobile", "contacts"])
  const { teamID = "" } = useParams()
  const { listPageCounts, scrollPositions } = useMobileNavigation()
  const [params, setParams] = useSearchParams()
  const queryText = params.get("q") ?? ""
  const [search, setSearch] = useState(queryText)
  // 团队名称复用团队列表的同一份缓存，读不到时回到通用标题。
  const { data: teams } = useResource(resourceKeys.teams(teamListQuery), (signal) =>
    listTeams(teamListQuery, signal),
  )
  const teamName = teams?.teams.find((team) => team.id === teamID)?.name

  useEffect(() => setSearch(queryText), [queryText])
  useEffect(() => {
    if (search === queryText) return
    // 停止输入后再查询，替换当前历史并保留团队列表返回来源。
    const timer = window.setTimeout(() => {
      const next = new URLSearchParams()
      if (search) next.set("q", search)
      // 搜索改变时重置目标查询的加载进度和滚动位置。
      if (search.trim() !== queryText.trim()) {
        const storageKey = `team:${teamID}:${search.trim()}`
        listPageCounts.delete(storageKey)
        scrollPositions.delete(storageKey)
      }
      setParams(next, { replace: true, state: { mobileBack: true } })
    }, 300)
    return () => window.clearTimeout(timer)
  }, [search, queryText, setParams, listPageCounts, scrollPositions, teamID])

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={teamName ?? t("contacts.teams")}
        backTo="/contacts/teams"
      />
      <div className="shrink-0 space-y-2 border-b px-4 py-3">
        <Label htmlFor="mobile-team-member-search">
          {t("contacts:search.teamMembers")}
        </Label>
        <Input
          id="mobile-team-member-search"
          type="search"
          className="min-h-11 md:text-base"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <MobileTeamMemberList
        key={`team:${teamID}:${queryText.trim()}`}
        teamID={teamID}
        queryText={queryText.trim()}
        searching={search !== queryText}
      />
    </section>
  )
}

/** 逐页读取团队成员，真人进成员资料，AI 员工进对话。 */
function MobileTeamMemberList({
  teamID,
  queryText,
  searching,
}: {
  teamID: string
  queryText: string
  searching: boolean
}) {
  const { t } = useTranslation("mobile")
  const { formatDateTime } = useDateTime()
  return (
    <MobilePagedList
      storageKey={`team:${teamID}:${queryText}`}
      searching={searching}
      labels={{
        loadError: t("teams.membersLoadError"),
        loadMoreError: t("contacts.loadMoreError"),
        empty: t("teams.membersEmpty"),
        allLoaded: t("contacts.allLoaded"),
      }}
      source={(page) => {
        const query = { query: queryText, page, pageSize: 50 }
        return {
          key: resourceKeys.teamMembers(teamID, query),
          load: (signal) => listTeamMembers(teamID, query, signal),
        }
      }}
      select={(data) => ({ items: data.members, page: data.page })}
    >
      {(members) => (
        <ul className="divide-y border-b">
          {members.map((member) => {
            const agent =
              member.identityType === OrganizationIdentityType.OrganizationIdentityTypeAgent
            const target = agent
              ? `/contacts/ai-employees/${member.agentId}/chat`
              : `/contacts/employees/${member.userId}`
            return (
              <li key={member.identityId}>
                <Link
                  to={target}
                  state={{ mobileBack: true }}
                  className="flex min-h-18 items-center gap-3 px-4 py-3 outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                >
                  <ProfileAvatar
                    name={member.displayName}
                    imageURL={member.avatarUrl}
                    fallback={agent ? "agent" : "person"}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[15px] font-medium">
                      {member.displayName}
                    </span>
                    <span className="mt-0.5 flex items-center gap-2">
                      <WorkStatusBadge status={member.workStatus} />
                      <span className="truncate text-xs text-muted-foreground">
                        {t("teams.joinedAt", {
                          time: formatDateTime(member.joinedAt),
                        })}
                      </span>
                    </span>
                  </span>
                  <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </MobilePagedList>
  )
}
