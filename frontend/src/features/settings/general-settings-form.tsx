/** 企业通用设置表单。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, updateOrganization, type Organization } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Switch } from "@/components/ui/switch"
import {
  createGeneralSettingsSchema,
  type GeneralSettingsFormValues,
} from "@/features/settings/general-settings-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 显示并修改当前企业通用设置。 */
export function GeneralSettingsForm({
  organization,
  onUpdated,
}: {
  organization: Organization
  onUpdated: (organization: Organization) => void
}) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const mounted = useRef(true)
  const [confirmingArbitraryURL, setConfirmingArbitraryURL] = useState(false)
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
      allowArbitraryUrl: organization.allowArbitraryUrl,
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
      if (!mounted.current) return
      form.reset({
        name: organization.name,
        allowArbitraryUrl: organization.allowArbitraryUrl,
      })
      onUpdated(organization)
      console.info("企业通用设置已更新", {
        organization_id: organization.id,
        allow_arbitrary_url: organization.allowArbitraryUrl,
      })
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
    <>
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
          <Controller
            name="allowArbitraryUrl"
            control={form.control}
            render={({ field }) => (
              <Field orientation="horizontal">
                <FieldLabel htmlFor={field.name}>
                  {t("general.form.allowArbitraryUrl")}
                </FieldLabel>
                <Switch
                  id={field.name}
                  name={field.name}
                  checked={field.value}
                  onBlur={field.onBlur}
                  onCheckedChange={(checked) => {
                    if (checked) {
                      setConfirmingArbitraryURL(true)
                      return
                    }
                    field.onChange(false)
                  }}
                  ref={field.ref}
                />
              </Field>
            )}
          />
        </FieldGroup>
        <div>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? (
              <LoaderCircleIcon className="animate-spin" />
            ) : null}
            {isSubmitting ? t("general.form.saving") : t("general.form.save")}
          </Button>
        </div>
      </form>

      <AlertDialog
        open={confirmingArbitraryURL}
        onOpenChange={setConfirmingArbitraryURL}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("general.confirm.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("general.confirm.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("general.confirm.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                form.setValue("allowArbitraryUrl", true, {
                  shouldDirty: true,
                })
              }}
            >
              {t("general.confirm.enable")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
