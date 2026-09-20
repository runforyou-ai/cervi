/** 企业通用设置表单。 */
import { useEffect, useMemo, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, updateOrganization, type Organization } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import {
  createGeneralSettingsSchema,
  type GeneralSettingsFormValues,
} from "@/features/settings/general-settings-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
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

  /** 保存企业通用设置。 */
  async function save(values: GeneralSettingsFormValues) {
    try {
      const organization = await updateOrganization(values)
      void invalidate(resourceKeys.identity())
      if (!mounted.current) return
      form.reset({ name: organization.name })
      toast.success(t("general.saveSuccess"))
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

  const { isSubmitting } = form.formState

  return (
    <form
      className="w-full max-w-xl space-y-9"
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup>
        <FormInputField
          name="name"
          control={form.control}
          label={t("general.form.name")}
          autoFocus
        />
      </FieldGroup>
      <div>
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
          {isSubmitting ? t("common:actions.saving") : t("common:actions.save")}
        </Button>
      </div>
    </form>
  )
}
