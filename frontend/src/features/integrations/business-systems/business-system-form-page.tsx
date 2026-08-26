/** 业务系统新增与编辑页。 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router"
import { toast } from "sonner"

import {
  createBusinessSystem,
  getBusinessSystem,
  isApiError,
  updateBusinessSystem,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Switch } from "@/components/ui/switch"
import {
  createBusinessSystemSchema,
  type BusinessSystemFormValues,
} from "@/features/integrations/business-systems/business-system-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const listPath = "/integrations/business-systems"

/** 编辑业务系统名称、地址和启用状态。 */
export function BusinessSystemFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const { businessSystemId = "" } = useParams()
  const [loading, setLoading] = useState(mode === "edit")
  const [loadError, setLoadError] = useState(false)
  const loadVersion = useRef(0)
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createBusinessSystemSchema({
        nameRequired: t("businessSystem.validation.nameRequired"),
        nameTooLong: t("businessSystem.validation.nameTooLong"),
        urlRequired: t("businessSystem.validation.urlRequired"),
        urlTooLong: t("businessSystem.validation.urlTooLong"),
        urlInvalid: t("businessSystem.validation.urlInvalid"),
      }),
    [t],
  )
  const form = useForm<BusinessSystemFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { name: "", url: "", enabled: true },
  })
  /** 读取待编辑的业务系统。 */
  const loadBusinessSystem = useCallback(async () => {
    const version = ++loadVersion.current
    setLoading(true)
    setLoadError(false)
    try {
      const businessSystem = await getBusinessSystem(businessSystemId)
      if (version !== loadVersion.current) return
      form.reset({
        name: businessSystem.name,
        url: businessSystem.url,
        enabled: businessSystem.enabled,
      })
    } catch (requestError) {
      if (version !== loadVersion.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("业务系统详情加载失败", {
        business_system_id: businessSystemId,
        error: requestError,
      })
      setLoadError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [businessSystemId, form, navigate])

  useEffect(() => {
    mounted.current = true
    if (mode === "edit") void loadBusinessSystem()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [loadBusinessSystem, mode])

  /** 创建或保存业务系统。 */
  async function save(values: BusinessSystemFormValues) {
    try {
      const businessSystem =
        mode === "create"
          ? await createBusinessSystem(values)
          : await updateBusinessSystem(businessSystemId, values)
      if (!mounted.current) return
      form.reset(values)
      console.info(
        mode === "create" ? "业务系统已创建" : "业务系统已保存",
        {
          business_system_id: businessSystem.id,
          enabled: businessSystem.enabled,
        },
      )
      toast.success(
        mode === "create"
          ? t("businessSystem.form.createSuccess")
          : t("businessSystem.form.updateSuccess"),
      )
      navigate(listPath)
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("业务系统保存失败", {
        business_system_id: businessSystemId,
        mode,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, ["name", "url"])
          : t("businessSystem.form.saveError"),
      )
    }
  }

  const title =
    mode === "create"
      ? t("businessSystem.form.createTitle")
      : t("businessSystem.form.editTitle")

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={title} />
      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("businessSystem.loading")}
          </div>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("businessSystem.form.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void loadBusinessSystem()}
            >
              {t("businessSystem.retry")}
            </Button>
          </div>
        ) : (
          <form
            className="w-full max-w-2xl space-y-9"
            onSubmit={form.handleSubmit(save)}
            noValidate
          >
            <FieldGroup>
              <FormInputField
                name="name"
                control={form.control}
                label={t("businessSystem.form.name")}
                autoFocus={mode === "create"}
              />
              <FormInputField
                name="url"
                control={form.control}
                label={t("businessSystem.form.url")}
                inputMode="url"
              />
              <Controller
                name="enabled"
                control={form.control}
                render={({ field }) => (
                  <Field orientation="horizontal">
                    <FieldLabel htmlFor={field.name}>
                      {t("businessSystem.form.enabled")}
                    </FieldLabel>
                    <Switch
                      id={field.name}
                      name={field.name}
                      checked={field.value}
                      onBlur={field.onBlur}
                      onCheckedChange={field.onChange}
                      ref={field.ref}
                    />
                  </Field>
                )}
              />
            </FieldGroup>
            <div className="flex items-center gap-2">
              <Button type="submit" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {form.formState.isSubmitting
                  ? t("businessSystem.form.saving")
                  : t("businessSystem.form.save")}
              </Button>
              <Button type="button" variant="outline" asChild>
                <Link to={listPath}>{t("businessSystem.form.cancel")}</Link>
              </Button>
            </div>
          </form>
        )}
      </PageContent>
    </div>
  )
}
