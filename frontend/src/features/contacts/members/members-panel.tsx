/** 同事目录面板：搜索在职同事和服务台并发起聊天。 */
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { OrganizationIdentityType, listColleagues, type ColleagueData } from "@/api"
import { ListToolbarSearch } from "@/components/list-toolbar"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { StatusBadge } from "@/components/status-badge"
import { useWorkspace } from "@/contexts/workspace-context"
import { ContactListSection } from "@/features/contacts/contact-list-section"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { useResource } from "@/hooks/use-resource"

/** 服务台在前、在职同事在后的目录，点击任一行进入与其单聊。 */
export function MembersPanel() {
  const { t } = useTranslation("contacts")
  const { formatDateTime } = useDateTime()
  const { identity } = useWorkspace()
  const navigate = useNavigate()
  const { setParameters, query, search, setSearch, currentPage } =
    useContactSearch()

  const list = useResource(
    resourceKeys.colleagues({ query, page: currentPage, pageSize: 50 }),
    () => listColleagues({ query, page: currentPage, pageSize: 50 }),
  )
  const colleagues = list.data?.colleagues ?? []
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
              cell: (colleague: ColleagueData) => {
                const serviceDesk =
                  colleague.identityType === OrganizationIdentityType.OrganizationIdentityTypeAgent
                return (
                  <ResourceRowIdentity
                    avatar={{
                      imageURL: colleague.avatarUrl,
                      name: colleague.displayName,
                      fallback: serviceDesk ? "agent" : "person",
                    }}
                    status={
                      colleague.identityId === identity.user.identityId
                        ? identity.user.workStatus
                        : colleague.workStatus
                    }
                    name={colleague.displayName}
                    secondary={colleague.teams.map((team) => team.name).join(t("teamSelect.separator"))}
                    badge={
                      serviceDesk ? (
                        <StatusBadge variant="muted">{t("list.serviceDesk")}</StatusBadge>
                      ) : undefined
                    }
                    description={
                      serviceDesk
                        ? colleague.responsibleName &&
                          t("list.responsible", { name: colleague.responsibleName })
                        : colleague.email
                    }
                  />
                )
              },
            },
            {
              key: "time",
              header: t("columns.addedAt"),
              cellClassName: "w-px whitespace-nowrap text-right text-muted-foreground",
              cell: (colleague) =>
                t("list.addedAt", { time: formatDateTime(colleague.createdAt) }),
            },
          ]}
          rows={colleagues}
          rowKey={(colleague) => colleague.identityId}
          empty={t("list.empty")}
          onRowActivate={(colleague) => navigate(`/chats?target=${colleague.identityId}`)}
          // 自己的行不可进入。
          canActivateRow={(colleague) => colleague.identityId !== identity.user.identityId}
        />
    </ContactListSection>
  )
}
