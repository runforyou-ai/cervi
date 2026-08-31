/** 通讯录新建成员、AI 员工和团队的共享弹窗。 */
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import type { ChannelOption, RoleData, Team } from "@/api"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { AgentForm } from "@/features/contacts/agents/agent-form"
import type { ContactScope } from "@/features/contacts/contact-scope"
import { ContactForm } from "@/features/contacts/external/contact-form"
import { MemberForm } from "@/features/contacts/members/member-form"
import { TeamForm } from "@/features/contacts/teams/team-form"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 按查询参数渲染新建成员或联系人、新建 AI 员工和新建团队弹窗。 */
export function ContactCreateDialogs({
  scope,
  channels,
  roles,
  teams,
  selectedTeam,
  searchParams,
  setParameters,
}: {
  scope: ContactScope
  channels: ChannelOption[]
  roles: RoleData[]
  teams: Team[]
  selectedTeam?: Team
  searchParams: URLSearchParams
  setParameters: (changes: Record<string, string | null>) => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const creating = searchParams.get("new") === "1"
  const creatingAgent = searchParams.get("newAgent") === "1"
  const creatingTeam = searchParams.get("newTeam") === "1"

  return (
    <>
      <Dialog
        open={creating}
        onOpenChange={(open) => !open && setParameters({ new: null })}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {t(scope !== "external" ? "members.create" : "detail.createTitle")}
            </DialogTitle>
            <DialogDescription>
              {t(
                scope !== "external"
                  ? "members.createDescription"
                  : "detail.createDescription",
              )}
            </DialogDescription>
          </DialogHeader>
          {scope !== "external" ? (
            <MemberForm
              teams={teams}
              roles={roles}
              defaultTeamIds={selectedTeam ? [selectedTeam.id] : []}
              onSaved={() => {
                setParameters({ new: null })
                void invalidate(resourceKeys.users())
                void invalidate(resourceKeys.teamMembers())
                void invalidate(resourceKeys.teamMemberCandidates())
                void invalidate(resourceKeys.teams())
                void invalidate(resourceKeys.roles())
                void invalidate(resourceKeys.roleMembers())
                void invalidate(resourceKeys.customerServiceAssignees())
              }}
              onCancel={() => setParameters({ new: null })}
            />
          ) : (
            <ContactForm
              channels={channels}
              onSaved={() => {
                setParameters({ new: null })
                void invalidate(resourceKeys.contacts())
              }}
              onCancel={() => setParameters({ new: null })}
            />
          )}
        </DialogContent>
      </Dialog>

      <Dialog
        open={creatingAgent}
        onOpenChange={(open) => !open && setParameters({ newAgent: null })}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("agents.create")}</DialogTitle>
            <DialogDescription>
              {t("agents.createDescription")}
            </DialogDescription>
          </DialogHeader>
          <AgentForm
            teams={teams}
            roles={roles}
            defaultTeamIds={selectedTeam ? [selectedTeam.id] : []}
            onSaved={() => {
              setParameters({ newAgent: null })
              void invalidate(resourceKeys.agents())
              void invalidate(resourceKeys.teamMembers())
              void invalidate(resourceKeys.teamMemberCandidates())
              void invalidate(resourceKeys.teams())
              void invalidate(resourceKeys.roles())
              void invalidate(resourceKeys.roleMembers())
              void invalidate(resourceKeys.customerServiceAssignees())
            }}
            onCancel={() => setParameters({ newAgent: null })}
          />
        </DialogContent>
      </Dialog>

      <Dialog
        open={creatingTeam}
        onOpenChange={(open) => !open && setParameters({ newTeam: null })}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("teams.create")}</DialogTitle>
            <DialogDescription>{t("teams.editDescription")}</DialogDescription>
          </DialogHeader>
          <TeamForm
            onSaved={(team) => {
              void invalidate(resourceKeys.teams())
              navigate(`/contacts/teams/${team.id}`, {
                replace: true,
              })
            }}
            onCancel={() => setParameters({ newTeam: null })}
          />
        </DialogContent>
      </Dialog>
    </>
  )
}
