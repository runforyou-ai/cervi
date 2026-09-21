/** 企业通用设置表单。 */
import { useEffect, useMemo, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, updateOrganization, type Organization } from "@/api"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  createGeneralSettingsSchema,
  type GeneralSettingsFormValues,
} from "@/features/settings/general-settings-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useAutoSave } from "@/hooks/use-auto-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { resolveServerURL } from "@/lib/server-url"
import { recoverSession } from "@/lib/session-navigation"

/** 显示并修改当前企业通用设置。 */
export function GeneralSettingsForm({
  organization,
}: {
  organization: Organization
}) {
  const { t } = useTranslation(["settings", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createGeneralSettingsSchema({
        nameRequired: t("general.validation.nameRequired"),
        nameTooLong: t("general.validation.nameTooLong"),
      }),
    [t],
  )
  const form = useForm<GeneralSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      name: organization.name,
    },
  })
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const markSaved = useAutoSave({ form, schema, save })
  const serverURL = useResource(resourceKeys.serverURL(), () => resolveServerURL())
  const domain = serverURL.data ? new URL(serverURL.data).host : ""

  /** 保存企业通用设置。 */
  async function save(values: GeneralSettingsFormValues) {
    try {
      const saved = await updateOrganization(values)
      void invalidate(resourceKeys.identity())
      if (!mounted.current) return
      const next = { name: saved.name }
      form.reset(next)
      markSaved(next)
    } catch (error) {
      if (!mounted.current) return
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("企业通用设置更新失败", {
        organization_id: organization.id,
        error,
      })
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["name"]))
        return
      }
      toast.error(t("general.saveError"))
    }
  }

  return (
    <form
      className="w-full"
      onSubmit={form.handleSubmit(save)}
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
        {/* 域名由部署或开通时确定，这里只读展示，可选中复制。 */}
        <Field>
          <FieldLabel htmlFor="general-domain">
            {t("general.form.domain")}
          </FieldLabel>
          <Input
            id="general-domain"
            value={domain}
            placeholder={serverURL.loading ? t("common:status.loading") : undefined}
            readOnly
            className="text-muted-foreground"
          />
        </Field>
      </FieldGroup>
    </form>
  )
}
