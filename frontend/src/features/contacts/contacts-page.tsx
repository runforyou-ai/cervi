/** 通讯录页面协调层：按范围渲染同事、团队、外部联系人和助理面板。 */
import { useEffect, type ReactNode } from "react"
import { useParams } from "react-router"

import { listChannelOptions } from "@/api"
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
  const channels = channelsResource.data ?? []
  const channelsError = channelsResource.error

  /** 渠道选项加载失败时记录日志，便于排查筛选项为空的原因。 */
  useEffect(() => {
    if (channelsError) {
      console.warn("通讯录渠道选项加载失败", channelsError)
    }
  }, [channelsError])

  return (
    <PageSplit paneVariant="nav" pane={<ContactScopeSidebar />}>
      {children ?? (scope === "assistants" ? (
        <AssistantsPanel />
      ) : scope === "employees" ? (
        <MembersPanel />
      ) : scope === "team" && teamId ? (
        <TeamPanel teamId={teamId} />
      ) : scope === "team" ? (
        <TeamListPanel />
      ) : (
        <ExternalContactsPanel channels={channels} />
      ))}
    </PageSplit>
  )
}
