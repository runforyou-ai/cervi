/** 移动端真人与 AI 员工目录的搜索、分页和返回恢复。 */
import { useEffect, useState } from "react"
import { ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useSearchParams } from "react-router"

import {
  listAgents,
  listUsers,
  UserStatus,
  type AgentListData,
  type AgentListItemData,
  type UserData,
  type UserListData,
} from "@/api"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader, MobileSearchBar } from "@/apps/mobile/mobile-page"
import { MobilePagedList } from "@/apps/mobile/mobile-paged-list"
import { ProfileAvatar } from "@/components/profile-avatar"
import { WorkStatusBadge } from "@/components/work-status"
import { resourceKeys } from "@/hooks/resource-keys"

type DirectoryKind = "employees" | "agents"

/** 目录用途：通讯录浏览，或从消息页发起新的 AI 会话。 */
type DirectoryPurpose = "browse" | "newConversation"

/** 防抖同步搜索条件，按目录类型加载真人或 AI 员工。 */
export function MobileDirectoryPage({
  kind,
  purpose = "browse",
}: {
  kind: DirectoryKind
  purpose?: DirectoryPurpose
}) {
  const { t } = useTranslation(["mobile", "inbox"])
  const location = useLocation()
  const { listPageCounts, scrollPositions } = useMobileNavigation()
  const [params, setParams] = useSearchParams()
  const queryText = params.get("q") ?? ""
  const [search, setSearch] = useState(queryText)
  const newConversation = purpose === "newConversation"
  const listKey = newConversation ? `${kind}:new` : kind

  useEffect(() => setSearch(queryText), [queryText])
  useEffect(() => {
    if (search === queryText) return
    // 停止输入后再查询，替换当前历史并保留通讯录返回来源。
    const timer = window.setTimeout(() => {
      const next = new URLSearchParams()
      if (search) next.set("q", search)
      // 搜索改变时重置目标查询的加载进度和滚动位置。
      if (search.trim() !== queryText.trim()) {
        const storageKey = `${listKey}:${search.trim()}`
        listPageCounts.delete(storageKey)
        scrollPositions.delete(storageKey)
      }
      setParams(next, { replace: true, state: location.state })
    }, 300)
    return () => window.clearTimeout(timer)
  }, [
    search, queryText, setParams, location.state,
    listPageCounts, scrollPositions, listKey,
  ])

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t(
          newConversation ? "inbox:newAgentConversation" : `contacts.${kind}`,
        )}
        backTo={newConversation ? "/inbox" : "/contacts"}
      />
      <MobileSearchBar
        label={t(kind === "agents" ? "agents.search" : "contacts.search")}
        value={search}
        onChange={setSearch}
      />
      <MobileDirectoryList
        key={`${listKey}:${queryText.trim()}`}
        kind={kind}
        listKey={listKey}
        newConversation={newConversation}
        queryText={queryText.trim()}
        searching={search !== queryText}
      />
    </section>
  )
}

/** 逐页读取目录成员，行内展示头像、名称和辅助信息。 */
function MobileDirectoryList({
  kind,
  listKey,
  newConversation,
  queryText,
  searching,
}: {
  kind: DirectoryKind
  listKey: string
  newConversation: boolean
  queryText: string
  searching: boolean
}) {
  const { t } = useTranslation("mobile")
  const agents = kind === "agents"
  return (
    <MobilePagedList<UserData | AgentListItemData, UserListData | AgentListData>
      storageKey={`${listKey}:${queryText}`}
      searching={searching}
      labels={{
        loadError: t(agents ? "agents.loadError" : "contacts.loadError"),
        loadMoreError: t(agents ? "agents.loadMoreError" : "contacts.loadMoreError"),
        empty: t(agents ? "agents.empty" : "contacts.empty"),
        allLoaded: t(agents ? "agents.allLoaded" : "contacts.allLoaded"),
      }}
      source={(page) => {
        const query = {
          query: queryText,
          status: UserStatus.UserStatusActive,
          page,
          pageSize: 50,
        }
        return {
          key: agents ? resourceKeys.agents(query) : resourceKeys.users(query),
          load: () => (agents ? listAgents(query) : listUsers(query)),
        }
      }}
      select={(data) => ({
        items: "agents" in data ? data.agents : data.users,
        page: data.page,
      })}
    >
      {(members) => (
        <ul className="divide-y border-b">
          {members.map((member) => (
            <li key={member.id}>
              <Link
                to={
                  agents
                    ? `/contacts/ai-employees/${member.id}/chat`
                    : `/contacts/employees/${member.id}`
                }
                replace={newConversation}
                state={{ mobileBack: true }}
                className="flex min-h-18 items-center gap-3 px-4 py-3 outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
              >
                <ProfileAvatar
                  name={member.displayName}
                  imageURL={member.avatarUrl}
                  fallback={agents ? "agent" : "person"}
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[15px] font-medium">
                    {member.displayName}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {"email" in member ? (
                      member.email
                    ) : (
                      <WorkStatusBadge status={member.workStatus} />
                    )}
                  </span>
                </span>
                <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </MobilePagedList>
  )
}
