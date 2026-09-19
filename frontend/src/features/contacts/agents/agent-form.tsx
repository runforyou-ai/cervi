/** 新建 AI 员工表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, useWatch, type Control } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  AgentExecutionMode,
  FilePurpose,
  RoleKind,
  createAgent,
  isApiError,
  type RoleData,
  type AgentData,
} from "@/api"
import { AgentBehaviorSummary } from "@/components/agent-behavior-summary"
import { FormInputField } from "@/components/form/form-input-field"
import { ImagePicker } from "@/components/image-picker"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { AgentModelField } from "@/features/contacts/agents/agent-model-field"
import { parseAgentModelSelection } from "@/features/contacts/agents/agent-model-selection"
import {
  createAgentSchema,
  type AgentFormValues,
} from "@/features/contacts/agents/agent-schema"
import { useContactInvalidator } from "@/features/contacts/use-contact-invalidator"
import { useFormLifetime } from "@/hooks/use-form-lifetime"
import { usePendingImageUpload } from "@/hooks/use-pending-image-upload"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { RoleSelectField } from "@/features/contacts/role-select-field"

/** 创建 AI 员工，可同时设置头像。 */
export function AgentForm({
  roles,
  defaultTeamIds = [],
  onSaved,
  onCancel,
}: {
  roles: RoleData[]
  defaultTeamIds?: string[]
  onSaved: (agent: AgentData) => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const invalidateContact = useContactInvalidator()
  const schema = useMemo(
    () =>
      createAgentSchema({
        nameRequired: t("agents.validation.nameRequired"),
        nameInvalid: t("agents.validation.nameInvalid"),
        roleRequired: t("members.validation.roleRequired"),
        modelRequired: t("agents.validation.modelRequired"),
        instructionTooLong: t("agents.validation.instructionTooLong"),
      }),
    [t],
  )
  const assignableRoles = roles.filter(
    (role) => role.kind !== RoleKind.RoleKindAdmin,
  )
  const defaultRoleID =
    assignableRoles.find((role) => role.kind === RoleKind.RoleKindMember)?.id ??
    assignableRoles[0]?.id ??
    ""
  const form = useForm<AgentFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      displayName: "",
      roleId: defaultRoleID,
      teamIds: defaultTeamIds,
      execution: {
        mode: AgentExecutionMode.AgentExecutionModeManaged,
        managed: {
          modelSelection: "",
          systemInstruction: "",
          knowledgeBaseIds: [],
        },
      },
    },
  })
  const avatar = usePendingImageUpload({
    purpose: FilePurpose.FilePurposeAgentAvatar,
    onError: (error) => {
      console.warn("上传 AI 员工头像失败", error)
      if (!recoverSession(error, navigate)) toast.error(t("avatar.uploadError"))
    },
  })
  const { mounted, dirty } = useFormLifetime(
    form.formState.isDirty || avatar.pending !== null,
  )
  const selectedRoleId = useWatch({ control: form.control, name: "roleId" })
  const selectedRole = assignableRoles.find((role) => role.id === selectedRoleId)

  /** 上传待保存的头像后提交 AI 员工表单。 */
  async function submit(values: AgentFormValues) {
    let uploadingAvatar = false
    try {
      const model = parseAgentModelSelection(
        values.execution.managed.modelSelection,
      )
      uploadingAvatar = Boolean(avatar.pending && !avatar.pending.fileID)
      const avatarFileId = await avatar.ensureUploaded()
      uploadingAvatar = false
      const created = await createAgent({
        displayName: values.displayName,
        roleId: values.roleId,
        teamIds: values.teamIds,
        avatarFileId,
        execution: {
          mode: values.execution.mode,
          managed: {
            ...model,
            systemInstruction: values.execution.managed.systemInstruction,
            knowledgeBaseIds: values.execution.managed.knowledgeBaseIds,
          },
        },
      })
      void invalidateContact("agent")
      if (!mounted.current) return
      toast.success(t("agents.form.created"))
      dirty.current = false
      form.reset(values)
      avatar.clear()
      onSaved(created)
    } catch (error) {
      // 上传失败已由共享上传回调提示，保存只处理资料提交错误。
      if (uploadingAvatar) return
      if (!mounted.current || recoverSession(error, navigate)) return
      console.warn("创建 AI 员工失败", { error })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "displayName",
              "roleId",
              "execution",
              "providerId",
              "modelIdentifier",
              "systemInstruction",
              "knowledgeBaseIds",
              "teamIds",
            ])
          : t("agents.form.networkError"),
      )
    }
  }

  return (
    <form
      className="w-full max-w-2xl space-y-9"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup>
        <Field>
          <FieldLabel>{t("avatar.label")}</FieldLabel>
          <ImagePicker
            imageURL={avatar.pending?.previewURL}
            fallback="agent"
            label={t("avatar.choose")}
            disabled={form.formState.isSubmitting}
            loading={avatar.pending?.status === "uploading"}
            onSelect={avatar.select}
          />
        </Field>
        <FormInputField
          name="displayName"
          id="agent-create-name"
          control={form.control}
          label={t("agents.form.name")}
          autoFocus
          disabled={form.formState.isSubmitting}
        />
        <Controller
          name="roleId"
          control={form.control}
          render={({ field, fieldState }) => (
            <RoleSelectField
              {...field}
              id={field.name}
              required
              disabled={form.formState.isSubmitting}
              aria-invalid={fieldState.invalid}
              roles={assignableRoles}
              hint={t(
                selectedRole?.kind === RoleKind.RoleKindCustomerService
                  ? "agents.roleHint.customerService"
                  : "agents.roleHint.member",
              )}
            />
          )}
        />
        {selectedRole?.agentBehavior ? (
          <Field>
            <FieldLabel>{t("agents.execution.behavior")}</FieldLabel>
            <AgentBehaviorSummary behavior={selectedRole.agentBehavior} />
          </Field>
        ) : null}
        <AgentManagedExecutionFields
          control={form.control}
          disabled={form.formState.isSubmitting}
        />
      </FieldGroup>
      <div className="flex items-center gap-2">
        <Button type="submit" disabled={form.formState.isSubmitting}>
          {form.formState.isSubmitting
            ? tCommon("actions.saving")
            : tCommon("actions.create")}
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={form.formState.isSubmitting}
          onClick={onCancel}
        >
          {tCommon("actions.cancel")}
        </Button>
      </div>
    </form>
  )
}

/** 渲染平台托管执行配置字段。 */
function AgentManagedExecutionFields({
  control,
  disabled,
}: {
  disabled: boolean
  control: Control<AgentFormValues>
}) {
  const { t } = useTranslation("contacts")
  return (
    <>
      <AgentModelField
        control={control}
        name="execution.managed.modelSelection"
        disabled={disabled}
      />
      <Controller
        name="execution.managed.systemInstruction"
        control={control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="agent-system-instruction">
              {t("agents.execution.instruction")}
            </FieldLabel>
            <Textarea
              {...field}
              id="agent-system-instruction"
              rows={6}
              disabled={disabled}
              aria-invalid={fieldState.invalid}
            />
            <FieldDescription>
              {t("agents.execution.instructionHelp")}
            </FieldDescription>
          </Field>
        )}
      />
    </>
  )
}
