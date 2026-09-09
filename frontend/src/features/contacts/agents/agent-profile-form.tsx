/** AI 员工基本资料表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  RoleKind,
  UserStatus,
  isApiError,
  updateAgent,
  type AgentData,
  type RoleData,
  type Team,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import {
  selectableWorkStatuses,
  workStatusLabel,
} from "@/components/work-status"
import { RoleSelectField } from "@/features/contacts/role-select-field"
import { TeamCheckboxField } from "@/features/contacts/team-checkbox-field"
import {
  createAgentProfileSchema,
  type AgentProfileFormValues,
} from "@/features/contacts/agents/agent-schema"
import { useFormLifetime } from "@/hooks/use-form-lifetime"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 单独保存 AI 员工名称、角色、工作状态和所属团队。 */
export function AgentProfileForm({
  agent,
  roles,
  teams,
  onSaved,
  onCancel,
}: {
  agent: AgentData
  roles: RoleData[]
  teams: Team[]
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation(["contacts", "common"])
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createAgentProfileSchema({
        nameRequired: t("agents.validation.nameRequired"),
        roleRequired: t("members.validation.roleRequired"),
      }),
    [t],
  )
  const form = useForm<AgentProfileFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      displayName: agent.displayName,
      roleId: agent.role.id,
      workStatus: agent.workStatus,
      teamIds: agent.teams.map((team) => team.id),
    },
  })
  const { mounted, dirty } = useFormLifetime(form.formState.isDirty)

  // 资料刷新时同步未修改的表单，保留正在编辑的草稿。
  useEffect(() => {
    if (dirty.current) return
    form.reset({
      displayName: agent.displayName,
      roleId: agent.role.id,
      workStatus: agent.workStatus,
      teamIds: agent.teams.map((team) => team.id),
    })
  }, [agent, dirty, form])

  /** 提交基本资料并保留其他页签的编辑内容。 */
  async function submit(values: AgentProfileFormValues) {
    try {
      await updateAgent(agent.id, values)
      onSaved()
      if (!mounted.current) return
      dirty.current = false
      form.reset(values)
      console.info("AI 员工基本资料已保存", { agent_id: agent.id })
      toast.success(t("agents.form.saved"))
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      console.warn("保存 AI 员工基本资料失败", { agent_id: agent.id, error })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "displayName",
              "roleId",
              "workStatus",
              "teamIds",
            ])
          : t("agents.form.networkError"),
      )
    }
  }

  return (
    <form className="space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup>
        <FormInputField
          name="displayName"
          id="agent-profile-name"
          control={form.control}
          label={t("agents.form.name")}
          disabled={form.formState.isSubmitting}
        />
        <Controller
          name="roleId"
          control={form.control}
          render={({ field, fieldState }) => (
            <RoleSelectField
              {...field}
              id="agent-profile-role"
              required
              aria-invalid={fieldState.invalid}
              disabled={form.formState.isSubmitting}
              roles={roles.filter(
                (role) => role.kind !== RoleKind.RoleKindAdmin,
              )}
            />
          )}
        />
        <Controller
          name="workStatus"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="agent-profile-work-status" required>
                {t("columns.workStatus")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id="agent-profile-work-status"
                required
                aria-invalid={fieldState.invalid}
                disabled={
                  form.formState.isSubmitting ||
                  agent.status !== UserStatus.UserStatusActive
                }
              >
                {selectableWorkStatuses.map((status) => (
                  <option key={status} value={status}>
                    {workStatusLabel(status, tCommon)}
                  </option>
                ))}
              </NativeSelect>
            </Field>
          )}
        />
        <Controller
          name="teamIds"
          control={form.control}
          render={({ field }) => (
            <TeamCheckboxField
              teams={teams}
              label={t("agents.form.teams")}
              emptyMessage={t("agents.form.noTeams")}
              value={field.value}
              onChange={field.onChange}
              onBlur={field.onBlur}
              disabled={form.formState.isSubmitting}
            />
          )}
        />
      </FieldGroup>
      <div className="flex items-center gap-2">
        <Button type="submit" disabled={form.formState.isSubmitting}>
          {t(
            form.formState.isSubmitting
              ? "common:actions.saving"
              : "common:actions.save",
          )}
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={form.formState.isSubmitting}
          onClick={onCancel}
        >
          {t("common:actions.cancel")}
        </Button>
      </div>
    </form>
  )
}
