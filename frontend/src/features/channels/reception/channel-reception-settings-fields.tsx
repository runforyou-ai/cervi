/** 各消息渠道共用的接待设置字段和候选项加载。 */
import { useEffect } from "react"
import {
  Controller,
  type Control,
  type FieldValues,
  type Path,
} from "react-hook-form"
import { useTranslation } from "react-i18next"
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
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { Field, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import type { ChannelReceptionSettingsFormValues } from "@/features/channels/reception/channel-reception-schema"

type ReceptionTargetName = keyof ChannelReceptionSettingsFormValues

const receptionOptionPageSize = 100

const routingChoices = [
  ChannelRoutingTargetType.ChannelRoutingTargetTypePublicQueue,
  ChannelRoutingTargetType.ChannelRoutingTargetTypeTeam,
  ChannelRoutingTargetType.ChannelRoutingTargetTypeMember,
] as const

/** 显示一个不带分组边框的接待目标字段。 */
function ReceptionTargetField<
  TValues extends FieldValues & ChannelReceptionSettingsFormValues,
>({
  name,
  control,
  teams,
  members,
}: {
  name: ReceptionTargetName
  control: Control<TValues>
  teams: Team[]
  members: MemberOption[]
}) {
  const { t } = useTranslation("channels")
  const isFallback = name === "fallbackTarget"

  return (
    <Controller
      name={`${name}.type` as Path<TValues>}
      control={control}
      render={({ field: typeField, fieldState: typeFieldState }) => (
        <Controller
          name={`${name}.id` as Path<TValues>}
          control={control}
          render={({ field: idField, fieldState: idFieldState }) => {
            const targetType = typeField.value as ChannelRoutingTarget["type"]
            const invalid = typeFieldState.invalid || idFieldState.invalid
            return (
              <Field data-invalid={invalid}>
                <FieldLabel htmlFor={`${name}-type`}>
                  {t(
                    isFallback ? "routing.fallback" : "routing.newConversation",
                  )}
                </FieldLabel>
                <NativeSelect
                  {...typeField}
                  id={`${name}-type`}
                  value={targetType}
                  aria-invalid={typeFieldState.invalid}
                  onChange={(event) => {
                    typeField.onChange(event)
                    idField.onChange("")
                  }}
                >
                  {routingChoices.map((choice) => (
                    <option key={choice} value={choice}>
                      {t(
                        `routing.${isFallback ? "fallbackTypes" : "newConversationTypes"}.${choice}`,
                      )}
                    </option>
                  ))}
                </NativeSelect>
                {targetType !==
                ChannelRoutingTargetType.ChannelRoutingTargetTypePublicQueue ? (
                  <div className="mt-3 flex w-full flex-col gap-2">
                    <FieldLabel htmlFor={`${name}-id`} required>
                      {isFallback
                        ? targetType ===
                          ChannelRoutingTargetType.ChannelRoutingTargetTypeTeam
                          ? t("routing.targetLabels.fallback.team")
                          : t("routing.targetLabels.fallback.member")
                        : targetType ===
                            ChannelRoutingTargetType.ChannelRoutingTargetTypeTeam
                          ? t("routing.targetLabels.newConversation.team")
                          : t("routing.targetLabels.newConversation.member")}
                    </FieldLabel>
                    <NativeSelect
                      {...idField}
                      id={`${name}-id`}
                      value={idField.value as string}
                      required
                      aria-invalid={idFieldState.invalid}
                    >
                      <option value="">{t("routing.select")}</option>
                      {targetType ===
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
          }}
        />
      )}
    />
  )
}

/** 分页读取全部团队接待候选项。 */
async function listAllTeams() {
  const teams: Team[] = []
  let page = 1
  let pages = 1
  do {
    const output = await listTeams({ page, pageSize: receptionOptionPageSize })
    teams.push(...output.teams)
    pages = Math.ceil(output.page.total / receptionOptionPageSize)
    page += 1
  } while (page <= pages)
  return teams
}

/** 分页读取全部成员接待候选项。 */
async function listAllMemberOptions() {
  const members: MemberOption[] = []
  let page = 1
  let pages = 1
  do {
    const output = await listMemberOptions({
      page,
      pageSize: receptionOptionPageSize,
    })
    members.push(...output.members)
    pages = Math.ceil(output.page.total / receptionOptionPageSize)
    page += 1
  } while (page <= pages)
  return members
}

/** 渲染可被不同渠道表单复用的接待设置字段。 */
export function ChannelReceptionSettingsFields<
  TValues extends FieldValues & ChannelReceptionSettingsFormValues,
>({ control }: { control: Control<TValues> }) {
  const { t } = useTranslation("channels")
  const { data: options, error } = useResource(
    resourceKeys.channelReceptionOptions(),
    async () => {
      const [teams, members] = await Promise.all([
        listAllTeams(),
        listAllMemberOptions(),
      ])
      return { teams, members }
    },
    { staleTime: 0 },
  )
  const teams = options?.teams ?? []
  const members = options?.members ?? []

  /** 候选项加载失败时提示用户。 */
  useEffect(() => {
    if (error) {
      console.warn("渠道接待候选项加载失败", error)
      toast.error(t("routing.loadError"))
    }
  }, [error, t])

  return (
    <>
      {(["newConversationTarget", "fallbackTarget"] as const).map((name) => (
        <ReceptionTargetField
          key={name}
          name={name}
          control={control}
          teams={teams}
          members={members}
        />
      ))}
    </>
  )
}
