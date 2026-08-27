/** 通讯录页面协调层：加载共享目录数据并按范围渲染面板。 */
import { useEffect } from "react"
import { useParams, useSearchParams } from "react-router"

import { listChannelOptions, listRoles, listTeams } from "@/api"
import { PageSplit } from "@/components/page-split"
import { AgentsPanel } from "@/features/contacts/agents/agents-panel"
import { ContactScopeSidebar } from "@/features/contacts/contact-scope-sidebar"
import { ExternalContactsPanel } from "@/features/contacts/external/external-contacts-panel"
import { MembersPanel } from "@/features/contacts/members/members-panel"
import { TeamPanel } from "@/features/contacts/teams/team-panel"
import { type ContactScope } from "@/features/contacts/contact-scope"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

export type { ContactScope }

/** 按分类列出通讯录。 */
export function ContactsPage({ scope }: { scope: ContactScope }) {
  const { teamId = "" } = useParams()
  const [searchParams] = useSearchParams()
  const deleted = scope === "external" && searchParams.get("view") === "trash"
  const channelId = searchParams.get("channelId") ?? ""

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

  /** 目录数据加载失败时记录日志，便于排查侧栏为空的原因。 */
  useEffect(() => {
    if (catalogError) {
      console.warn("通讯录筛选数据加载失败", catalogError)
    }
  }, [catalogError])

  return (
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <ContactScopeSidebar
          scope={scope}
          deleted={deleted}
          channelId={channelId}
          channels={channels}
          teamId={teamId}
          teams={teams}
        />
      }
    >
      {scope === "employees" ? (
        <MembersPanel channels={channels} roles={roles} teams={teams} />
      ) : scope === "agents" ? (
        <AgentsPanel channels={channels} roles={roles} teams={teams} />
      ) : scope === "team" ? (
        <TeamPanel
          channels={channels}
          roles={roles}
          teams={teams}
          teamId={teamId}
        />
      ) : (
        <ExternalContactsPanel
          channels={channels}
          roles={roles}
          teams={teams}
        />
      )}
    </PageSplit>
  )
}
