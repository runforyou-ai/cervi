/** 移动端真人与 AI 员工目录的搜索、分页和返回恢复。 */
import { ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

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
import { useListSearchParams } from "@/hooks/use-list-search-params"

type DirectoryKind = "employees" | "agents"

/** 防抖同步搜索条件，按目录类型加载真人或 AI 员工。 */
export function MobileDirectoryPage({ kind }: { kind: DirectoryKind }) {
  const { t } = useTranslation("mobile")
  const { listPageCounts, scrollPositions } = useMobileNavigation()
  // 检索词变化时重置目标查询的加载进度和滚动位置。
  const { query: queryText, search, setSearch } = useListSearchParams({
    onQueryChange: (query) => {
      const storageKey = `${kind}:${query}`
      listPageCounts.delete(storageKey)
      scrollPositions.delete(storageKey)
    },
  })

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t(`contacts.${kind}`)} backTo="/contacts" />
      <MobileSearchBar
        label={t(kind === "agents" ? "agents.search" : "contacts.search")}
        value={search}
        onChange={setSearch}
      />
      <MobileDirectoryList
        key={`${kind}:${queryText.trim()}`}
        kind={kind}
        queryText={queryText.trim()}
        searching={search !== queryText}
      />
    </section>
  )
}

/** 逐页读取目录成员，行内展示头像、名称和辅助信息。 */
function MobileDirectoryList({
  kind,
  queryText,
  searching,
}: {
  kind: DirectoryKind
  queryText: string
  searching: boolean
}) {
  const { t } = useTranslation("mobile")
  const agents = kind === "agents"
  return (
    <MobilePagedList<UserData | AgentListItemData, UserListData | AgentListData>
      storageKey={`${kind}:${queryText}`}
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
