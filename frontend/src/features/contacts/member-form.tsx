/** 新建企业成员表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  RoleKind,
  createUser,
  isApiError,
  type RoleData,
  type Team,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { roleDisplayName } from "@/features/roles/role-labels"
import {
  createMemberSchema,
  type MemberFormValues,
} from "@/features/contacts/member-schema"

/** 创建企业成员。 */
export function MemberForm({
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
  const defaultRoleID =
    roles.find((role) => role.kind === RoleKind.RoleKindMember)?.id ??
    roles[0]?.id ??
    ""
  const schema = useMemo(
    () =>
      createMemberSchema(
        {
          nameRequired: t("members.validation.nameRequired"),
          emailRequired: t("members.validation.emailRequired"),
          emailInvalid: t("members.validation.emailInvalid"),
          passwordRequired: t("members.validation.passwordRequired"),
          passwordTooShort: t("members.validation.passwordTooShort"),
          passwordTooLong: t("members.validation.passwordTooLong"),
          roleRequired: t("members.validation.roleRequired"),
        },
        false,
      ),
    [t],
  )
  const form = useForm<MemberFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      displayName: "",
      email: "",
      password: "",
      roleId: defaultRoleID,
      teamIds: defaultTeamIds,
    },
  })

  useEffect(() => {
    if (!form.getValues("roleId") && defaultRoleID) {
      form.setValue("roleId", defaultRoleID)
    }
  }, [defaultRoleID, form])

  /** 提交企业成员表单。 */
  async function submit(values: MemberFormValues) {
    try {
      await createUser({
        displayName: values.displayName,
        email: values.email,
        password: values.password,
        roleId: values.roleId,
        teamIds: values.teamIds,
      })
      toast.success(t("members.form.created"))
      onSaved()
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("保存企业成员失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "displayName",
              "email",
              "password",
              "roleId",
              "teamIds",
            ])
          : t("members.form.networkError"),
      )
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup className="gap-5">
        <FormInputField
          name="displayName"
          control={form.control}
          label={t("members.form.name")}
          autoFocus
        />
        <FormInputField
          name="email"
          control={form.control}
          label={t("members.form.email")}
          type="email"
        />
        <FormInputField
          name="password"
          control={form.control}
          label={t("members.form.password")}
          type="password"
        />
        <Controller
          name="roleId"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("members.form.role")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              >
                {roles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {roleDisplayName(role, tCommon)}
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
            <Field>
              <FieldLabel>{t("members.form.teams")}</FieldLabel>
              {teams.length === 0 ? (
                <FieldDescription>{t("members.form.noTeams")}</FieldDescription>
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
              ? t("members.form.saving")
              : t("members.form.save")}
          </Button>
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("members.form.cancel")}
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
