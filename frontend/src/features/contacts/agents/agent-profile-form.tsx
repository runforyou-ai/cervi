/** AI 员工基本资料表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  FilePurpose,
  RoleKind,
  UserStatus,
  isApiError,
  updateAgent,
  type AgentData,
  type RoleData,
  type Team,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { SwitchCardField } from "@/components/form/switch-card-field"
import { ImagePicker } from "@/components/image-picker"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import {
  selectableWorkStatuses,
  workStatusLabel,
} from "@/components/work-status"
import { RoleSelectField } from "@/features/contacts/role-select-field"
import { TeamSelectField } from "@/features/contacts/team-select-field"
import {
  createAgentProfileSchema,
  type AgentProfileFormValues,
} from "@/features/contacts/agents/agent-schema"
import { useFormLifetime } from "@/hooks/use-form-lifetime"
import { usePendingImageUpload } from "@/hooks/use-pending-image-upload"
import { apiErrorMessage } from "@/lib/form-errors"
import { useAutoSave } from "@/hooks/use-auto-save"
import { recoverSession } from "@/lib/session-navigation"

/** 单独保存 AI 员工头像、名称、角色、工作状态和所属团队。 */
export function AgentProfileForm({
  agent,
  roles,
  teams,
  onSaved,
}: {
  agent: AgentData
  roles: RoleData[]
  teams: Team[]
  onSaved: () => void
}) {
  const { t } = useTranslation(["contacts", "common"])
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createAgentProfileSchema({
        nameRequired: t("agents.validation.nameRequired"),
        nameInvalid: t("agents.validation.nameInvalid"),
        roleRequired: t("members.validation.roleRequired"),
      }),
    [t],
  )
  const form = useForm<AgentProfileFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      displayName: agent.displayName,
      roleId: agent.role.id,
      workStatus: agent.workStatus,
      teamIds: agent.teams.map((team) => team.id),
      handlesCustomers: agent.handlesCustomers,
    },
  })
  const avatar = usePendingImageUpload({
    purpose: FilePurpose.FilePurposeAgentAvatar,
    onError: (error) => {
      console.warn("上传 AI 员工头像失败", { agent_id: agent.id, error })
      if (!recoverSession(error, navigate)) toast.error(t("avatar.uploadError"))
    },
  })
  const { mounted, dirty, discarded } = useFormLifetime(
    form.formState.isDirty || avatar.pending !== null,
  )

  // 资料刷新时同步未修改的表单，保留正在编辑的草稿。
  useEffect(() => {
    if (dirty.current) return
    form.reset({
      displayName: agent.displayName,
      roleId: agent.role.id,
      workStatus: agent.workStatus,
      teamIds: agent.teams.map((team) => team.id),
      handlesCustomers: agent.handlesCustomers,
    })
  }, [agent, dirty, form])

  /** 提交基本资料和待保存的头像，并保留其他页签的编辑内容。 */
  const { markSaved } = useAutoSave({ form, schema, save: submit, discarded })

  async function submit(values: AgentProfileFormValues) {
    let uploadingAvatar = false
    try {
      uploadingAvatar = Boolean(avatar.pending && !avatar.pending.fileID)
      const avatarFileId = await avatar.ensureUploaded()
      uploadingAvatar = false
      await updateAgent(agent.id, { ...values, avatarFileId })
      onSaved()
      if (!mounted.current) return true
      dirty.current = false
      form.reset(values)
      markSaved(values)
      avatar.clear()
      return true
    } catch (error) {
      // 上传失败已由共享上传回调提示，保存只处理资料提交错误。
      if (uploadingAvatar) return false
      // 离开页面后提交的改动失败时同样提示。
      if (recoverSession(error, navigate)) return false
      console.warn("保存 AI 员工基本资料失败", { agent_id: agent.id, error })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "displayName",
              "roleId",
              "workStatus",
              "teamIds",
              "handlesCustomers",
            ])
          : t("agents.form.networkError"),
      )
      return false
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup>
        <Field>
          <FieldLabel>{t("avatar.label")}</FieldLabel>
          <ImagePicker
            imageURL={avatar.pending?.previewURL || agent.avatarUrl}
            fallback="agent"
            label={t("avatar.choose")}
            disabled={form.formState.isSubmitting}
            loading={avatar.pending?.status === "uploading"}
            onSelect={avatar.select}
          />
        </Field>
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
          name="teamIds"
          control={form.control}
          render={({ field }) => (
            <TeamSelectField
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
          name="handlesCustomers"
          control={form.control}
          render={({ field }) => (
            <SwitchCardField
              id="agent-profile-handles-customers"
              name={field.name}
              label={t("agents.form.handlesCustomers")}
              description={t("agents.form.handlesCustomersHelp")}
              checked={field.value}
              disabled={form.formState.isSubmitting}
              onBlur={field.onBlur}
              onCheckedChange={field.onChange}
              ref={field.ref}
            />
          )}
        />
      </FieldGroup>
    </form>
  )
}
