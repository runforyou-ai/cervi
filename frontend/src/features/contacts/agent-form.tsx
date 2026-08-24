/** 新建 AI 员工表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { createAgent, isApiError, type Team } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { AgentModelField } from "@/features/contacts/agent-model-field"
import { parseAgentModelSelection } from "@/features/contacts/agent-model-selection"
import {
  createAgentSchema,
  type AgentFormValues,
} from "@/features/contacts/agent-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 创建 AI 员工。 */
export function AgentForm({
  teams,
  defaultTeamIds = [],
  onSaved,
  onCancel,
}: {
  teams: Team[]
  defaultTeamIds?: string[]
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createAgentSchema({
        nameRequired: t("agents.validation.nameRequired"),
        modelRequired: t("agents.validation.modelRequired"),
        instructionRequired: t("agents.validation.instructionRequired"),
        instructionTooLong: t("agents.validation.instructionTooLong"),
      }),
    [t],
  )
  const form = useForm<AgentFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      displayName: "",
      teamIds: defaultTeamIds,
      modelSelection: "",
      systemInstruction: "",
    },
  })

  /** 提交 AI 员工表单。 */
  async function submit(values: AgentFormValues) {
    try {
      const model = parseAgentModelSelection(values.modelSelection)
      const created = await createAgent({
        displayName: values.displayName,
        teamIds: values.teamIds,
        capability: {
          ...model,
          systemInstruction: values.systemInstruction,
        },
      })
      console.info("AI 员工已创建", {
        agent_id: created.id,
        provider_id: created.capability.providerId,
        model_identifier: created.capability.modelIdentifier,
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
              "providerId",
              "modelIdentifier",
              "systemInstruction",
              "teamIds",
            ])
          : t("agents.form.networkError"),
      )
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup className="gap-5">
        <FormInputField
          name="displayName"
          control={form.control}
          label={t("agents.form.name")}
          autoFocus
        />
        <AgentModelField control={form.control} name="modelSelection" />
        <Controller
          name="systemInstruction"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="agent-system-instruction" required>
                {t("agents.capability.instruction")}
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
          name="teamIds"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel>{t("agents.form.teams")}</FieldLabel>
              {teams.length === 0 ? (
                <FieldDescription>{t("agents.form.noTeams")}</FieldDescription>
              ) : (
                <div className="grid gap-2 rounded-md border p-3 sm:grid-cols-2">
                  {teams.map((team) => (
                    <label
                      key={team.id}
                      className="flex items-center gap-2 text-sm"
                    >
                      <input
                        type="checkbox"
                        className="size-4 accent-primary"
                        checked={field.value.includes(team.id)}
                        onChange={(event) =>
                          field.onChange(
                            event.target.checked
                              ? [...field.value, team.id]
                              : field.value.filter((id) => id !== team.id),
                          )
                        }
                      />
                      <span>{team.name}</span>
                    </label>
                  ))}
                </div>
              )}
            </Field>
          )}
        />
        <div className="flex items-center gap-2 pt-2">
          <Button type="submit" disabled={form.formState.isSubmitting}>
            {form.formState.isSubmitting
              ? t("agents.form.saving")
              : t("agents.form.save")}
          </Button>
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("agents.form.cancel")}
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
