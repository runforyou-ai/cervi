/** 新建 AI 员工表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, type Control } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  AgentExecutionMode,
  RoleKind,
  createAgent,
  isApiError,
  type RoleData,
  type Team,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { AgentKnowledgeField } from "@/features/contacts/agents/agent-knowledge-field"
import { AgentModelField } from "@/features/contacts/agents/agent-model-field"
import { parseAgentModelSelection } from "@/features/contacts/agents/agent-model-selection"
import {
  createAgentSchema,
  type AgentFormValues,
} from "@/features/contacts/agents/agent-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { RoleSelectField } from "@/features/contacts/role-select-field"
import { TeamCheckboxField } from "@/features/contacts/team-checkbox-field"

/** 创建 AI 员工。 */
export function AgentForm({
  teams,
  roles,
  defaultTeamIds = [],
  onSaved,
  onCancel,
}: {
  teams: Team[]
  roles: RoleData[]
  defaultTeamIds?: string[]
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createAgentSchema({
        nameRequired: t("agents.validation.nameRequired"),
        roleRequired: t("members.validation.roleRequired"),
        modelRequired: t("agents.validation.modelRequired"),
        instructionRequired: t("agents.validation.instructionRequired"),
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
  /** 提交 AI 员工表单。 */
  async function submit(values: AgentFormValues) {
    try {
      const model = parseAgentModelSelection(
        values.execution.managed.modelSelection,
      )
      const created = await createAgent({
        displayName: values.displayName,
        roleId: values.roleId,
        teamIds: values.teamIds,
        execution: {
          mode: values.execution.mode,
          managed: {
            ...model,
            systemInstruction: values.execution.managed.systemInstruction,
            knowledgeBaseIds: values.execution.managed.knowledgeBaseIds,
          },
        },
      })
      console.info("AI 员工已创建", {
        agent_id: created.id,
        execution_mode: created.execution.mode,
        revision_id: created.execution.revisionId,
        provider_id: created.execution.managed.providerId,
        model_identifier: created.execution.managed.modelIdentifier,
      })
      toast.success(t("agents.form.created"))
      onSaved()
    } catch (error) {
      if (recoverSession(error, navigate)) return
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
      className="space-y-9"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup className="gap-5">
        <FormInputField
          name="displayName"
          control={form.control}
          label={t("agents.form.name")}
          autoFocus
        />
        <Controller
          name="roleId"
          control={form.control}
          render={({ field, fieldState }) => (
            <RoleSelectField
              {...field}
              id={field.name}
              aria-invalid={fieldState.invalid}
              roles={assignableRoles}
            />
          )}
        />
        <AgentManagedExecutionFields control={form.control} />
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
            />
          )}
        />
      </FieldGroup>
      <div className="flex items-center gap-2">
        <Button type="submit" disabled={form.formState.isSubmitting}>
          {form.formState.isSubmitting
            ? tCommon("actions.saving")
            : tCommon("actions.save")}
        </Button>
        <Button type="button" variant="outline" onClick={onCancel}>
          {tCommon("actions.cancel")}
        </Button>
      </div>
    </form>
  )
}

/** 渲染平台托管执行配置字段。 */
function AgentManagedExecutionFields({
  control,
}: {
  control: Control<AgentFormValues>
}) {
  const { t } = useTranslation("contacts")
  return (
    <>
      <AgentModelField
        control={control}
        name="execution.managed.modelSelection"
      />
      <Controller
        name="execution.managed.systemInstruction"
        control={control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="agent-system-instruction" required>
              {t("agents.execution.instruction")}
            </FieldLabel>
            <Textarea
              {...field}
              id="agent-system-instruction"
              rows={6}
              required
              aria-invalid={fieldState.invalid}
            />
          </Field>
        )}
      />
      <Controller
        name="execution.managed.knowledgeBaseIds"
        control={control}
        render={({ field }) => (
          <Field>
            <FieldLabel>{t("agents.execution.knowledgeBases")}</FieldLabel>
            <AgentKnowledgeField value={field.value} onChange={field.onChange} />
          </Field>
        )}
      />
    </>
  )
}
