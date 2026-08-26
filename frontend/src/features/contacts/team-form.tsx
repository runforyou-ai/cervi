/** 新建和编辑团队表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { createTeam, isApiError, updateTeam, type Team } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { useWorkspaceTabDirty } from "@/contexts/workspace-tab-lifecycle"
import { recoverSession } from "@/lib/session-navigation"
import { apiErrorMessage } from "@/lib/form-errors"
import {
  createTeamSchema,
  type TeamFormValues,
} from "@/features/contacts/team-schema"

/** 保存新团队或现有团队。 */
export function TeamForm({
  team,
  onSaved,
  onCancel,
}: {
  team?: Team
  onSaved: (team: Team) => void
  onCancel?: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createTeamSchema({
        nameRequired: t("teams.validation.nameRequired"),
        nameTooLong: t("teams.validation.nameTooLong"),
        descriptionTooLong: t("teams.validation.descriptionTooLong"),
      }),
    [t],
  )
  const form = useForm<TeamFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: team?.name ?? "",
      description: team?.description ?? "",
    },
  })
  useWorkspaceTabDirty(
    form.formState.isDirty && !form.formState.isSubmitting,
  )

  useEffect(() => {
    if (!team) form.setFocus("name")
  }, [form, team])

  /** 提交团队表单。 */
  async function submit(values: TeamFormValues) {
    try {
      const saved = team
        ? await updateTeam(team.id, values)
        : await createTeam(values)
      toast.success(t(team ? "teams.form.updated" : "teams.form.created"))
      onSaved(saved)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("保存团队失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["name", "description"])
          : t("teams.form.networkError"),
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
          name="name"
          control={form.control}
          label={t("teams.form.name")}
        />
        <Controller
          name="description"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>
                {t("teams.form.description")}
              </FieldLabel>
              <Textarea
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              />
            </Field>
          )}
        />
      </FieldGroup>
      <div className="flex items-center gap-2">
        <Button type="submit" disabled={form.formState.isSubmitting}>
          {form.formState.isSubmitting
            ? t("teams.form.saving")
            : t("teams.form.save")}
        </Button>
        {onCancel ? (
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("teams.form.cancel")}
          </Button>
        ) : null}
      </div>
    </form>
  )
}
