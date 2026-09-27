/** 工作区通用设置表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { updateOrganization, type Organization } from "@/api"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  createGeneralSettingsSchema,
  type GeneralSettingsFormValues,
} from "@/features/settings/general-settings-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useFormSave } from "@/hooks/use-form-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { resolveServerURL } from "@/lib/server-url"
import { enterWorkspace, workspaceHref } from "@/lib/workspace-route"

/** 显示并修改当前工作区的名称和标识，标识修改后转到新的工作区地址。 */
export function GeneralSettingsForm({
  organization,
}: {
  organization: Organization
}) {
  const { t } = useTranslation(["settings", "common"])
  const invalidate = useResourceInvalidator()
  const schema = useMemo(
    () =>
      createGeneralSettingsSchema({
        nameRequired: t("general.validation.nameRequired"),
        nameTooLong: t("general.validation.nameTooLong"),
        slugRequired: t("general.validation.slugRequired"),
        slugInvalid: t("general.validation.slugInvalid"),
      }),
    [t],
  )
  const form = useForm<GeneralSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      name: organization.name,
      slug: organization.slug,
    },
  })
  const { submit } = useFormSave({
    form,
    schema,
    autoSave: true,
    save: async (values) => {
      const saved = await updateOrganization(values)
      void invalidate(resourceKeys.identity())
      void invalidate(resourceKeys.workspaces())
      return saved
    },
    savedValues: (saved) => ({ name: saved.name, slug: saved.slug }),
    onSaved: (saved) => {
      if (saved.slug !== organization.slug) {
        enterWorkspace(saved.slug, "/settings/general", { replace: true })
      }
    },
    errorMessage: t("general.saveError"),
    errorFields: ["name", "slug"],
    logLabel: "工作区通用设置更新",
  })
  const serverURL = useResource(resourceKeys.serverURL(), () => resolveServerURL())
  const address = serverURL.data ? `${serverURL.data.replace(/\/+$/, "")}/#${workspaceHref(organization.slug, "/")}` : ""

  return (
    <form
      className="w-full"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup>
        <Controller
          name="name"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("general.form.name")}
              </FieldLabel>
              <Input
                {...field}
                id={field.name}
                autoComplete="organization"
                aria-invalid={fieldState.invalid}
                required
              />
            </Field>
          )}
        />
        <Controller
          name="slug"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("general.form.slug")}
              </FieldLabel>
              <Input
                {...field}
                id={field.name}
                autoCapitalize="none"
                autoCorrect="off"
                aria-invalid={fieldState.invalid}
                required
              />
            </Field>
          )}
        />
        {/* 访问地址由部署地址和工作区标识组成，这里只读展示，可选中复制。 */}
        <Field>
          <FieldLabel htmlFor="general-address">
            {t("general.form.address")}
          </FieldLabel>
          <Input
            id="general-address"
            value={address}
            readOnly
            className="text-muted-foreground"
          />
        </Field>
      </FieldGroup>
    </form>
  )
}
