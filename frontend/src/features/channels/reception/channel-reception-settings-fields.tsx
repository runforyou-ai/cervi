/** 各消息渠道共用的接待设置字段和候选项加载。 */
import { useEffect, useState } from "react"
import {
  Controller,
  type Control,
  type FieldValues,
  type Path,
} from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ChannelRoutingTargetType,
  OrganizationIdentityType,
  listMemberOptions,
  listTeams,
  type ChannelRoutingTarget,
  type MemberOption,
  type Team,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { Field, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import type { ChannelReceptionSettingsFormValues } from "@/features/channels/reception/channel-reception-schema"

type ReceptionTargetName = keyof ChannelReceptionSettingsFormValues

const routingChoices = [
  ChannelRoutingTargetType.ChannelRoutingTargetTypePublicQueue,
  ChannelRoutingTargetType.ChannelRoutingTargetTypeTeam,
  ChannelRoutingTargetType.ChannelRoutingTargetTypeMember,
] as const

/** 显示一个不带分组边框的接待目标字段。 */
function ReceptionTargetField({
  name,
  target,
  invalid,
  teams,
  members,
  onChange,
  onBlur,
}: {
  name: ReceptionTargetName
  target: ChannelRoutingTarget
  invalid: boolean
  teams: Team[]
  members: MemberOption[]
  onChange: (target: ChannelRoutingTarget) => void
  onBlur: () => void
}) {
  const { t } = useTranslation("channels")
  const isFallback = name === "fallbackTarget"

  return (
    <Field data-invalid={invalid}>
      <FieldLabel htmlFor={`${name}-type`}>
        {t(isFallback ? "routing.fallback" : "routing.newConversation")}
      </FieldLabel>
      <NativeSelect
        id={`${name}-type`}
        value={target.type}
        aria-invalid={invalid}
        onBlur={onBlur}
        onChange={(event) =>
          onChange({
            type: event.target.value as ChannelRoutingTarget["type"],
            id: "",
          })
        }
      >
        {routingChoices.map((choice) => (
          <option key={choice} value={choice}>
            {t(
              `routing.${isFallback ? "fallbackTypes" : "newConversationTypes"}.${choice}`,
            )}
          </option>
        ))}
      </NativeSelect>
      {target.type !==
      ChannelRoutingTargetType.ChannelRoutingTargetTypePublicQueue ? (
        <div className="mt-3 flex w-full flex-col gap-2">
          <FieldLabel htmlFor={`${name}-id`} required>
            {isFallback
              ? target.type ===
                ChannelRoutingTargetType.ChannelRoutingTargetTypeTeam
                ? t("routing.targetLabels.fallback.team")
                : t("routing.targetLabels.fallback.member")
              : target.type ===
                  ChannelRoutingTargetType.ChannelRoutingTargetTypeTeam
                ? t("routing.targetLabels.newConversation.team")
                : t("routing.targetLabels.newConversation.member")}
          </FieldLabel>
          <NativeSelect
            id={`${name}-id`}
            value={target.id}
            required
            aria-invalid={invalid}
            onBlur={onBlur}
            onChange={(event) =>
              onChange({ ...target, id: event.target.value })
            }
          >
            <option value="">{t("routing.select")}</option>
            {target.type ===
            ChannelRoutingTargetType.ChannelRoutingTargetTypeTeam
              ? teams.map((team) => (
                  <option key={team.id} value={team.id}>
                    {team.name}
                  </option>
                ))
              : members.map((member) => (
                  <option key={member.id} value={member.id}>
                    {member.displayName}（
                    {t(
                      member.type ===
                      OrganizationIdentityType.OrganizationIdentityTypeAgent
                        ? "routing.agent"
                        : "routing.person",
                    )}
                    ）
                  </option>
                ))}
          </NativeSelect>
        </div>
      ) : null}
    </Field>
  )
}

/** 渲染可被不同渠道表单复用的接待设置字段。 */
export function ChannelReceptionSettingsFields<
  TValues extends FieldValues & ChannelReceptionSettingsFormValues,
>({ control }: { control: Control<TValues> }) {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const [teams, setTeams] = useState<Team[]>([])
  const [members, setMembers] = useState<MemberOption[]>([])

  useEffect(() => {
    let active = true
    void Promise.all([
      listTeams({ pageSize: 100 }),
      listMemberOptions({ pageSize: 100 }),
    ])
      .then(([teamList, memberList]) => {
        if (!active) return
        setTeams(teamList.teams)
        setMembers(memberList.members)
      })
      .catch((error: unknown) => {
        if (!active) return
        if (recoverSession(error, navigate)) return
        console.warn("渠道接待候选项加载失败", error)
        setTeams([])
        setMembers([])
        toast.error(t("routing.loadError"))
      })
    return () => {
      active = false
    }
  }, [navigate, t])

  return (
    <>
      {(["newConversationTarget", "fallbackTarget"] as const).map((name) => (
        <Controller
          key={name}
          name={name as Path<TValues>}
          control={control}
          render={({ field, fieldState }) => (
            <ReceptionTargetField
              name={name}
              target={field.value as ChannelRoutingTarget}
              invalid={fieldState.invalid}
              teams={teams}
              members={members}
              onChange={field.onChange}
              onBlur={field.onBlur}
            />
          )}
        />
      ))}
    </>
  )
}
