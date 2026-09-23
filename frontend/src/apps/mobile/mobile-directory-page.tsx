/** 移动端同事目录的搜索、分页和返回恢复。 */
import { ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { listUsers, UserStatus } from "@/api"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader, MobileSearchBar } from "@/apps/mobile/mobile-page"
import { MobilePagedList } from "@/apps/mobile/mobile-paged-list"
import { ProfileAvatar } from "@/components/profile-avatar"
import { resourceKeys } from "@/hooks/resource-keys"
import { useListSearchParams } from "@/hooks/use-list-search-params"

/** 防抖同步搜索条件，加载在职同事。 */
export function MobileDirectoryPage() {
  const { t } = useTranslation(["mobile", "contacts"])
  const { listPageCounts, scrollPositions } = useMobileNavigation()
  // 检索词变化时重置目标查询的加载进度和滚动位置。
  const { query: queryText, search, setSearch } = useListSearchParams({
    onQueryChange: (query) => {
      const storageKey = `employees:${query}`
      listPageCounts.delete(storageKey)
      scrollPositions.delete(storageKey)
    },
  })

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t("contacts:scopes.employees")} backTo="/contacts" />
      <MobileSearchBar
        label={t("contacts.search")}
        value={search}
        onChange={setSearch}
      />
      <MobileDirectoryList
        key={`employees:${queryText.trim()}`}
        queryText={queryText.trim()}
        searching={search !== queryText}
      />
    </section>
  )
}

/** 逐页读取同事，行内展示头像、姓名、所属团队和邮箱。 */
function MobileDirectoryList({
  queryText,
  searching,
}: {
  queryText: string
  searching: boolean
}) {
  const { t } = useTranslation(["mobile", "contacts"])
  return (
    <MobilePagedList
      storageKey={`employees:${queryText}`}
      searching={searching}
      labels={{
        loadError: t("contacts.loadError"),
        loadMoreError: t("contacts.loadMoreError"),
        empty: t("contacts.empty"),
        allLoaded: t("contacts.allLoaded"),
      }}
      source={(page) => {
        const query = {
          query: queryText,
          status: UserStatus.UserStatusActive,
          page,
          pageSize: 50,
        }
        return {
          key: resourceKeys.users(query),
          load: () => listUsers(query),
        }
      }}
      select={(data) => ({ items: data.users, page: data.page })}
    >
      {(users) => (
        <ul className="divide-y border-b">
          {users.map((user) => (
            <li key={user.id}>
              <Link
                to={`/contacts/employees/${user.id}`}
                state={{ mobileBack: true }}
                className="flex min-h-18 items-center gap-3 px-4 py-3 outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
              >
                <ProfileAvatar name={user.displayName} imageURL={user.avatarUrl} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[15px] font-medium">
                    {user.displayName}
                    {user.teams.length ? (
                      <span className="font-normal text-muted-foreground">
                        {" · "}
                        {user.teams
                          .map((team) => team.name)
                          .join(t("contacts:teamSelect.separator"))}
                      </span>
                    ) : null}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {user.email}
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
