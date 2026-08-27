/** 窄视口下的通讯录范围切换选择器。 */
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import type { ChannelOption, Team } from "@/api"
import { NativeSelect } from "@/components/ui/native-select"
import type { ContactScope } from "@/features/contacts/contact-scope"

/** 在窄视口用下拉选择切换通讯录范围。 */
export function ContactScopeMobileSelect({
  scope,
  teamId = "",
  channelId = "",
  teams,
  channels,
}: {
  scope: ContactScope
  teamId?: string
  channelId?: string
  teams: Team[]
  channels: ChannelOption[]
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const mobileScope =
    scope === "employees"
      ? "employees"
      : scope === "agents"
        ? "agents"
        : scope === "team"
          ? `team:${teamId}`
          : channelId
            ? `channel:${channelId}`
            : "external"

  /** 窄视口下切换联系人范围。 */
  function changeMobileScope(value: string) {
    if (value === "employees") {
      navigate("/contacts/employees")
    } else if (value === "agents") {
      navigate("/contacts/ai-employees")
    } else if (value.startsWith("team:")) {
      navigate(`/contacts/teams/${value.slice("team:".length)}`)
    } else if (value.startsWith("channel:")) {
      navigate(`/contacts/external?channelId=${value.slice("channel:".length)}`)
    } else {
      navigate("/contacts/external")
    }
  }

  return (
    <div className="w-full md:hidden">
      <NativeSelect
        className="h-8 w-full"
        aria-label={t("scopeNavigation")}
        value={mobileScope}
        onChange={(event) => changeMobileScope(event.target.value)}
      >
        <option value="employees">{t("scopes.employees")}</option>
        <option value="agents">{t("scopes.agents")}</option>
        {teams.map((team) => (
          <option key={team.id} value={`team:${team.id}`}>
            {t("scopes.teams")} · {team.name}
          </option>
        ))}
        <option value="external">
          {t("scopes.external")} · {t("all")}
        </option>
        {channels.map((channel) => (
          <option key={channel.id} value={`channel:${channel.id}`}>
            {t("scopes.external")} · {channel.name}
          </option>
        ))}
      </NativeSelect>
    </div>
  )
}
