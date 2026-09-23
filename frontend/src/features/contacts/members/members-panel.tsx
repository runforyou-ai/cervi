/** 同事目录面板：搜索在职同事、查看资料并发起聊天。 */
import { useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { UserStatus, listUsers, type UserData } from "@/api"
import { ListToolbarSearch } from "@/components/list-toolbar"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { useWorkspace } from "@/contexts/workspace-context"
import { ContactListSection } from "@/features/contacts/contact-list-section"
import { MemberProfileSheet } from "@/features/contacts/members/member-profile-sheet"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 在职同事的列表、资料面板和发消息入口。 */
export function MembersPanel() {
  const { t } = useTranslation("contacts")
  const { identity } = useWorkspace()
  const navigate = useNavigate()
  const { setParameters, query, search, setSearch, currentPage, selected } =
    useContactSearch()
  const status = UserStatus.UserStatusActive

  const list = useResource(
    resourceKeys.users({ query, status, roleId: "", page: currentPage, pageSize: 50 }),
    () => listUsers({ query, status, roleId: "", page: currentPage, pageSize: 50 }),
  )
  const users = list.data?.users ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  // 保持引用稳定，供资料面板的读取失败处理依赖。
  const closeProfile = useCallback(
    () => setParameters({ selected: null }),
    [setParameters],
  )

  return (
    <>
      <ContactListSection
        title={t("scopes.employees")}
        description={t("scopeDescriptions.employees")}
        scope="employees"
        toolbar={
          <ListToolbarSearch
            value={search}
            aria-label={t("search.employees")}
            onChange={(event) => setSearch(event.target.value)}
          />
        }
        list={list}
        page={page}
        setParameters={setParameters}
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "employee",
              header: t("columns.employeeName"),
              cellClassName: "min-w-0",
              cell: (user: UserData) => (
                <ResourceRowIdentity
                  avatar={{ imageURL: user.avatarUrl, name: user.displayName }}
                  status={
                    user.id === identity.user.id
                      ? identity.user.workStatus
                      : user.workStatus
                  }
                  name={user.displayName}
                  secondary={user.teams.map((team) => team.name).join(t("teamSelect.separator"))}
                  description={user.email}
                />
              ),
            },
          ]}
          rows={users}
          rowKey={(user) => user.id}
          empty={t("list.empty")}
          onRowActivate={(user) => setParameters({ selected: user.id })}
          // 自己没有发消息。
          rowActions={(user) =>
            user.identityId !== identity.user.identityId
              ? [
                  {
                    key: "message",
                    label: t("sendMessage"),
                    onSelect: () => navigate(`/chats?target=${user.identityId}`),
                  },
                ]
              : []
          }
        />
      </ContactListSection>

      <MemberProfileSheet userId={selected} onClose={closeProfile} />
    </>
  )
}
