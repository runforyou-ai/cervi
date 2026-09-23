/** 同事目录面板：搜索在职同事并发起聊天。 */
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { UserStatus, listUsers, type UserData } from "@/api"
import { ListToolbarSearch } from "@/components/list-toolbar"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { useWorkspace } from "@/contexts/workspace-context"
import { ContactListSection } from "@/features/contacts/contact-list-section"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 在职同事列表，点击同事进入与其单聊。 */
export function MembersPanel() {
  const { t } = useTranslation("contacts")
  const { identity } = useWorkspace()
  const navigate = useNavigate()
  const { setParameters, query, search, setSearch, currentPage } =
    useContactSearch()
  const status = UserStatus.UserStatusActive

  const list = useResource(
    resourceKeys.users({ query, status, roleId: "", page: currentPage, pageSize: 50 }),
    () => listUsers({ query, status, roleId: "", page: currentPage, pageSize: 50 }),
  )
  const users = list.data?.users ?? []
  const page = list.data?.page ?? { number: currentPage, size: 50, total: 0 }

  return (
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
          // 点击自己的行不跳转。
          onRowActivate={(user) => {
            if (user.identityId !== identity.user.identityId) {
              navigate(`/chats?target=${user.identityId}`)
            }
          }}
        />
    </ContactListSection>
  )
}
