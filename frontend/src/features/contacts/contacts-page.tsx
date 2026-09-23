/** 通讯录页面协调层：加载共享目录数据并按范围渲染面板。 */
import { useEffect, type ReactNode } from "react"
import { useParams } from "react-router"

import { listChannelOptions, listRoles, listTeams } from "@/api"
import { PageSplit } from "@/components/page-split"
import { AssistantsPanel } from "@/features/contacts/assistants/assistants-panel"
import { ContactScopeSidebar } from "@/features/contacts/contact-scope-sidebar"
import { ExternalContactsPanel } from "@/features/contacts/external/external-contacts-panel"
import { MembersPanel } from "@/features/contacts/members/members-panel"
import { TeamListPanel } from "@/features/contacts/teams/team-list-panel"
import { TeamPanel } from "@/features/contacts/teams/team-panel"
import { type ContactScope } from "@/features/contacts/contact-scope"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

export type { ContactScope }

/** 按分类列出通讯录，传入 children 时在同一布局中渲染该分类下的子页面。 */
export function ContactsPage({ scope, children }: { scope: ContactScope; children?: ReactNode }) {
  const { teamId = "" } = useParams()

  const channelsResource = useResource(
    resourceKeys.channelOptions(),
    () => listChannelOptions(),
    { staleTime: 0 },
  )
  const rolesResource = useResource(resourceKeys.roles(), () => listRoles())
  const teamsResource = useResource(resourceKeys.teams({ pageSize: 100 }), () =>
    listTeams({ pageSize: 100 }),
  )
  const channels = channelsResource.data ?? []
  const roles = rolesResource.data?.roles ?? []
  const teams = teamsResource.data?.teams ?? []
  const catalogError =
    channelsResource.error ?? rolesResource.error ?? teamsResource.error

  /** 目录数据加载失败时记录日志，便于排查筛选项为空的原因。 */
  useEffect(() => {
    if (catalogError) {
      console.warn("通讯录筛选数据加载失败", catalogError)
    }
  }, [catalogError])

  return (
    <PageSplit paneVariant="nav" pane={<ContactScopeSidebar />}>
      {children ?? (scope === "assistants" ? (
        <AssistantsPanel />
      ) : scope === "employees" ? (
        <MembersPanel channels={channels} roles={roles} teams={teams} />
      ) : scope === "team" && teamId ? (
        <TeamPanel roles={roles} teams={teams} teamId={teamId} />
      ) : scope === "team" ? (
        <TeamListPanel channels={channels} roles={roles} teams={teams} />
      ) : (
        <ExternalContactsPanel
          channels={channels}
          roles={roles}
          teams={teams}
        />
      ))}
    </PageSplit>
  )
}
