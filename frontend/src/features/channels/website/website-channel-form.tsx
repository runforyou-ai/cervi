/** 网站渠道基础信息表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ErrorKind,
  Locale,
  createWebsiteChannel,
  isApiError,
  recoverSession,
  updateWebsiteChannel,
  type WebsiteChannelSummary,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { Textarea } from "@/components/ui/textarea"
import {
  createWebsiteChannelSchema,
  type WebsiteChannelFormValues,
} from "@/features/channels/website/website-channel-schema"
import { apiErrorMessage } from "@/lib/form-errors"

/** 创建或修改网站渠道基础信息。 */
export function WebsiteChannelForm({
  channel,
  onUpdated,
}: {
  channel?: WebsiteChannelSummary
  onUpdated?: (value: WebsiteChannelSummary) => void
}) {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createWebsiteChannelSchema({
        nameRequired: t("validation.nameRequired"),
        nameTooLong: t("validation.nameTooLong"),
        descriptionTooLong: t("validation.descriptionTooLong"),
      }),
    [t]
  )
  const form = useForm<WebsiteChannelFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: channel?.name ?? "",
      description: channel?.description ?? "",
      defaultLocale: channel?.defaultLocale ?? Locale.LocaleChineseSimplified,
    },
  })

  /** 提交网站渠道基础信息。 */
  async function submit(values: WebsiteChannelFormValues) {
    try {
      if (channel) {
        const updated = await updateWebsiteChannel(channel.id, values)
        form.reset({
          name: updated.name,
          description: updated.description ?? "",
          defaultLocale: updated.defaultLocale,
        })
        onUpdated?.(updated)
        console.info("网站渠道已保存", { channel_id: channel.id })
        toast.success(t("form.saved"))
        return
      }

      await createWebsiteChannel(values)
      console.info("网站渠道已创建")
      navigate("/channels/website", { replace: true })
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      if (isApiError(error)) {
        if (error.kind === ErrorKind.ErrorKindNotFound) {
          console.warn("网站渠道不存在", { channel_id: channel?.id })
          navigate("/channels/website", { replace: true })
          return
        }
        console.warn("保存网站渠道失败", error)
        toast.error(
          apiErrorMessage(error, ["name", "description", "defaultLocale"])
        )
        return
      }
      console.warn("保存网站渠道失败", error)
      toast.error(t("form.networkError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form
      className="w-full max-w-2xl"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup>
        <FormInputField
          name="name"
          control={form.control}
          label={t("form.name")}
          autoFocus
        />

        <Controller
          name="description"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>
                {t("form.description")}
              </FieldLabel>
              <Textarea
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              />
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />

        <Controller
          name="defaultLocale"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("form.defaultLocale")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                required
                aria-invalid={fieldState.invalid}
              >
                <option value={Locale.LocaleChineseSimplified}>{t("locales.zhCN")}</option>
                <option value={Locale.LocaleEnglishUnitedStates}>{t("locales.enUS")}</option>
              </NativeSelect>
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />

        <div className="flex items-center gap-2">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("form.saving") : t("form.save")}
          </Button>
          <Button variant="outline" asChild>
            <Link to="/channels/website">{t("form.cancel")}</Link>
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
