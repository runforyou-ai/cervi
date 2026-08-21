/** 企业信息设置表单。 */
import { useEffect, useMemo, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  recoverSession,
  updateOrganization,
  type Organization,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import {
  createOrganizationSettingsSchema,
  type OrganizationSettingsFormValues,
} from "@/features/settings/organization-settings-schema"
import { apiErrorMessage } from "@/lib/form-errors"

/** 显示并修改当前企业名称。 */
export function OrganizationSettingsForm({
  organization,
  onUpdated,
}: {
  organization: Organization
  onUpdated: (organization: Organization) => void
}) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createOrganizationSettingsSchema({
        nameRequired: t("organization.validation.nameRequired"),
        nameTooLong: t("organization.validation.nameTooLong"),
      }),
    [t],
  )
  const form = useForm<OrganizationSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { name: organization.name },
  })

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 保存企业名称。 */
  async function save(values: OrganizationSettingsFormValues) {
    try {
      const organization = await updateOrganization(values)
      if (!mounted.current) return
      form.reset({ name: organization.name })
      onUpdated(organization)
      console.info("企业名称已更新", {
        organization_id: organization.id,
      })
      toast.success(t("organization.saveSuccess"))
    } catch (error) {
      if (!mounted.current) return
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("企业名称更新失败", {
        organization_id: organization.id,
        error,
      })
      if (isApiError(error)) {
        const message = error.fields.name
        if (message) {
          form.setError("name", { message }, { shouldFocus: true })
          return
        }
        toast.error(apiErrorMessage(error))
        return
      }
      toast.error(t("organization.saveError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form
      className="mt-6 w-full max-w-xl"
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup>
        <FormInputField
          name="name"
          control={form.control}
          label={t("organization.form.name")}
          autoFocus
        />
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
          {isSubmitting
            ? t("organization.form.saving")
            : t("organization.form.save")}
        </Button>
      </FieldGroup>
    </form>
  )
}
